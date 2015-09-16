package represent

import (
	"errors"
	"fmt"
	"strconv"

	log "github.com/tag1consulting/pipeviz/Godeps/_workspace/src/github.com/Sirupsen/logrus"
	"github.com/tag1consulting/pipeviz/Godeps/_workspace/src/github.com/mndrix/ps"
	"github.com/tag1consulting/pipeviz/interpret"
	"github.com/tag1consulting/pipeviz/represent/types"
)

var i2a = strconv.Itoa

/*
CoreGraph is the interface provided by pipeviz' main graph object.

It is a persistent/immutable datastructure: only the Merge() method is able
to change the graph, but that method returns a pointer to a new graph rather
than updating in-place. (These copies typically share structure for efficiency)

All other methods are read-only, and generally provide composable parts that
work together to facilitate the creation of larger traversals/queries.
*/
type CoreGraph interface {
	// Merge a message into the graph, returning a pointer to the new graph
	// that contains the resulting updates.
	Merge(msg interpret.Message) CoreGraph

	// Enumerates the outgoing edges from the ego vertex, limiting the result set
	// to those that pass the provided filter (if any).
	OutWith(egoId int, ef EFilter) (es []StandardEdge)

	// Enumerates the incoming edges from the ego vertex, limiting the result set
	// to those that pass the provided filter (if any).
	InWith(egoId int, ef EFilter) (es []StandardEdge)

	// Enumerates the successors (targets of outgoing edges) from the ego vertex,
	// limiting the result set to those that pass the provided edge and vertex
	// filters (if any).
	SuccessorsWith(egoId int, vef VEFilter) (vts []VertexTuple)

	// Enumerates the predecessors (sources of incoming edges) from the ego vertex,
	// limiting the result set to those that pass the provided edge and vertex
	// filters (if any).
	PredecessorsWith(egoId int, vef VEFilter) (vts []VertexTuple)

	// Enumerates the vertices that pass the provided vertex filter (if any).
	VerticesWith(vf VFilter) (vs []VertexTuple)

	// Gets the vertex tuple associated with a given id.
	Get(id int) (VertexTuple, error)

	// Returns the message id for the current version of the graph. The graph's
	// contents are guaranteed to represent the state resulting from a correct
	// in-order interpretation of the messages up to the id, inclusive.
	MsgId() uint64
}

// the main graph construct
type coreGraph struct {
	// TODO experiment with replacing with a hash array-mapped trie
	msgid   uint64
	vserial int
	vtuples ps.Map
	orphans edgeSpecSet // FIXME breaks immut
}

func NewGraph() CoreGraph {
	log.WithFields(log.Fields{
		"system": "engine",
	}).Debug("New coreGraph created")

	return &coreGraph{vtuples: ps.NewMap(), vserial: 0}
}

type (
	// A value indicating a vertex's type. For now, done as a string.
	VType types.VType
	// A value indicating an edge's type. For now, done as a string.
	EType types.EType
)

const (
	VTypeNone types.VType = ""
	ETypeNone types.EType = ""
)

// Used in queries to specify property k/v pairs.
type PropQ struct {
	K string
	V interface{}
}

type VertexTuple struct {
	id int
	v  types.Vtx
	ie ps.Map
	oe ps.Map
}

// Returns the numeric id of the vertex tuple.
func (vt VertexTuple) Id() int {
	return vt.id
}

// Returns the vertex data of the vertex tuple.
func (vt VertexTuple) Vertex() types.Vtx {
	return vt.v
}

// Returns the out-edges of the vertex tuple.
func (vt VertexTuple) OutEdges() []StandardEdge {
	var ret []StandardEdge

	vt.oe.ForEach(func(k string, v ps.Any) {
		ret = append(ret, v.(StandardEdge))
	})

	return ret
}

type Property struct {
	MsgSrc uint64      `json:"msgsrc"`
	Value  interface{} `json:"value"`
}

type veProcessingInfo struct {
	vt    VertexTuple
	es    EdgeSpecs
	msgid uint64
}

type edgeSpecSet []*veProcessingInfo

func (ess edgeSpecSet) EdgeCount() (i int) {
	for _, tuple := range ess {
		i = i + len(tuple.es)
	}
	return
}

// Clones the graph object, and the map pointers it contains.
func (g *coreGraph) clone() *coreGraph {
	var cp coreGraph
	cp = *g
	return &cp
}

func (g *coreGraph) MsgId() uint64 {
	return g.msgid
}

// the method to merge a message into the graph
func (og *coreGraph) Merge(msg interpret.Message) CoreGraph {
	// TODO use a buffering pool to minimize allocs
	var ess edgeSpecSet

	logEntry := log.WithFields(log.Fields{
		"system": "engine",
		"msgid":  msg.Id,
	})

	logEntry.Infof("Merging message %d into graph", msg.Id)

	g := og.clone()
	g.msgid = msg.Id

	// Process incoming elements from the message
	msg.Each(func(d interface{}) {
		// Split each input element into vertex and edge specs
		sds, err := Split(d, msg.Id)

		if err != nil {
			logEntry.WithField("err", err).Warnf("Error while splitting input element of type %T; discarding", d)
			return
		}

		// Ensure vertices are present
		var tuples []VertexTuple
		for _, sd := range sds {
			tuples = append(tuples, g.ensureVertex(msg.Id, sd))
		}

		// Collect edge specs for later processing
		for k, tuple := range tuples {
			ess = append(ess, &veProcessingInfo{
				vt:    tuple,
				es:    sds[k].EdgeSpecs,
				msgid: msg.Id,
			})
		}
	})
	logEntry.Infof("Splitting all message elements produced %d edge spec sets", len(ess))

	logEntry.Infof("Adding %d orphan edge spec sets from previous merges", len(g.orphans))
	// Reinclude the held-over set of orphans for edge (re-)resolutions
	var ess2 edgeSpecSet
	// TODO lots of things very wrong with this approach, but works for first pass
	for _, orphan := range g.orphans {
		// vertex ident failed; try again now that new vertices are present
		if orphan.vt.id == 0 {
			orphan.vt = g.ensureVertex(orphan.msgid, SplitData{orphan.vt.v, orphan.es})
		} else {
			// ensure we have latest version of vt
			vt, err := g.Get(orphan.vt.id)
			if err != nil {
				// but if that vid has gone away, forget about it completely
				logEntry.Infof("Orphan vid %d went away, discarding from orphan list", orphan.vt.id)
				continue
			}
			orphan.vt = vt
		}

		ess2 = append(ess2, orphan)
	}

	// Put orphan stuff first so that it's guaranteed to be overwritten on conflict
	ess = append(ess2, ess...)

	// All vertices processed. Now, process edges in passes, ensuring that each
	// pass diminishes the number of remaining edges. If it doesn't, the remaining
	// edges need to be attached to null-vertices of the appropriate type.
	//
	// This is a little wasteful, but it's the simplest way to let any possible
	// dependencies between edges work themselves out. It has provably incorrect
	// cases, however, and will need to be replaced.
	var ec, lec, pass int
	for ec = ess.EdgeCount(); ec != 0 && ec != lec; ec = ess.EdgeCount() {
		pass += 1
		lec = ec
		l2 := logEntry.WithFields(log.Fields{
			"pass":       pass,
			"edge-count": ec,
		})

		l2.Debug("Beginning edge resolution pass")
		for infokey, info := range ess {
			l3 := logEntry.WithFields(log.Fields{
				"vid":   info.vt.id,
				"vtype": info.vt.v.Typ(),
			})
			specs := info.es
			info.es = info.es[:0]
			for _, spec := range specs {
				l3.Debugf("Resolving EdgeSpec of type %T", spec)
				edge, success := Resolve(g, msg.Id, info.vt, spec)
				if success {
					l4 := l3.WithFields(log.Fields{
						"target-vid": edge.Target,
						"etype":      edge.EType,
					})

					l4.Debug("Edge resolved successfully")

					edge.Source = info.vt.id
					if edge.id == 0 {
						// new edge, allocate a new id for it
						g.vserial++
						edge.id = g.vserial
						l4.WithField("edge-id", edge.id).Debug("New edge created")
					} else {
						l4.WithField("edge-id", edge.id).Debug("Edge will merge over existing edge")
					}

					info.vt.oe = info.vt.oe.Set(i2a(edge.id), edge)
					g.vtuples = g.vtuples.Set(i2a(info.vt.id), info.vt)

					any, _ := g.vtuples.Lookup(i2a(edge.Target))

					tvt := any.(VertexTuple)
					tvt.ie = tvt.ie.Set(i2a(edge.id), edge)
					g.vtuples = g.vtuples.Set(i2a(tvt.id), tvt)
				} else {
					l3.Debug("Unsuccessful edge resolution; reattempt on next pass")
					// FIXME mem leaks if done this way...?
					info.es = append(info.es, spec)
				}
			}
			// set the processing info back into its original position in the slice
			ess[infokey] = info
		}
	}
	logEntry.WithField("passes", pass).Info("Edge resolution complete")

	g.orphans = g.orphans[:0]
	for _, info := range ess {
		if len(info.es) == 0 {
			continue
		}

		g.orphans = append(g.orphans, info)
	}
	logEntry.Infof("Adding %d orphan edge spec sets from previous merges", len(g.orphans))

	return g
}

// Ensures the vertex is present. Merges according to type-specific logic if
// it is present, otherwise adds the vertex.
//
// Either way, return value is the vid for the vertex.
func (g *coreGraph) ensureVertex(msgid uint64, sd SplitData) (final VertexTuple) {
	logEntry := log.WithFields(log.Fields{
		"system": "engine",
		"msgid":  msgid,
		"vtype":  sd.Vertex.Typ(),
	})

	logEntry.Debug("Performing vertex unification")
	vid := Identify(g, sd)

	if vid == 0 {
		logEntry.Debug("No match on unification, creating new vertex")
		final = VertexTuple{v: sd.Vertex, ie: ps.NewMap(), oe: ps.NewMap()}
		g.vserial += 1
		final.id = g.vserial
		g.vtuples = g.vtuples.Set(i2a(g.vserial), final)
		// TODO remove this - temporarily cheat here by promoting EnvLink resolution, since so much relies on it
		for _, spec := range sd.EdgeSpecs {
			switch spec.(type) {
			case interpret.EnvLink, SpecDatasetHierarchy:
				logEntry.Debugf("Doing early resolve on EdgeSpec of type %T", spec)
				edge, success := Resolve(g, msgid, final, spec)
				if success { // could fail if corresponding env not yet declared
					logEntry.WithField("target-vid", edge.Target).Debug("Early resolve succeeded")
					g.vserial += 1
					edge.id = g.vserial
					final.oe = final.oe.Set(i2a(edge.id), edge)

					// set edge in reverse direction, too
					any, _ := g.vtuples.Lookup(i2a(edge.Target))
					tvt := any.(VertexTuple)
					tvt.ie = tvt.ie.Set(i2a(edge.id), edge)
					g.vtuples = g.vtuples.Set(i2a(tvt.id), tvt)
					g.vtuples = g.vtuples.Set(i2a(final.id), final)
				} else {
					logEntry.Debug("Early resolve failed")
				}
			}
		}
	} else {
		logEntry.WithField("vid", vid).Debug("Unification resulted in match")
		ivt, _ := g.vtuples.Lookup(i2a(vid))
		vt := ivt.(VertexTuple)

		nu, err := vt.v.Merge(sd.Vertex)
		if err != nil {
			logEntry.WithFields(log.Fields{
				"vid": vid,
				"err": err,
			}).Warn("Merge of vertex properties returned an error; vertex will continue update into graph anyway")
		}

		final = VertexTuple{id: vid, ie: vt.ie, oe: vt.oe, v: nu}
		g.vtuples = g.vtuples.Set(i2a(vid), final)
	}

	return
}

// Gets the vtTuple for a given vertex id.
func (g *coreGraph) Get(id int) (VertexTuple, error) {
	if id > g.vserial {
		return VertexTuple{}, errors.New(fmt.Sprintf("Graph has only %d elements, no vertex yet exists with id %d", g.vserial, id))
	}

	vtx, exists := g.vtuples.Lookup(i2a(id))
	if exists {
		return vtx.(VertexTuple), nil
	} else {
		return VertexTuple{}, errors.New(fmt.Sprintf("No vertex exists with id %d at the present revision of the graph", id))
	}
}
