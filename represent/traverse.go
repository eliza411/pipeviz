package represent

import (
	"bytes"

	log "github.com/tag1consulting/pipeviz/Godeps/_workspace/src/github.com/Sirupsen/logrus"
	"github.com/tag1consulting/pipeviz/Godeps/_workspace/src/github.com/mndrix/ps"
	"github.com/tag1consulting/pipeviz/represent/types"
)

type EFilter interface {
	EType() types.EType
	EProps() []PropQ
}

type VFilter interface {
	VType() types.VType
	VProps() []PropQ
}

type VEFilter interface {
	EFilter
	VFilter
}

type edgeFilter struct {
	etype types.EType
	props []PropQ
}

type vertexFilter struct {
	vtype types.VType
	props []PropQ
}

type bothFilter struct {
	VFilter
	EFilter
}

// all temporary functions to just make query building a little easier for now
func (vf vertexFilter) VType() types.VType {
	return vf.vtype
}

func (vf vertexFilter) VProps() []PropQ {
	return vf.props
}

func (vf vertexFilter) EType() types.EType {
	return ETypeNone
}

func (vf vertexFilter) EProps() []PropQ {
	return nil
}

func (vf vertexFilter) and(ef EFilter) VEFilter {
	return bothFilter{vf, ef}
}

func (ef edgeFilter) VType() types.VType {
	return VTypeNone
}

func (ef edgeFilter) VProps() []PropQ {
	return nil
}

func (ef edgeFilter) EType() types.EType {
	return ef.etype
}

func (ef edgeFilter) EProps() []PropQ {
	return ef.props
}

func (ef edgeFilter) and(vf VFilter) VEFilter {
	return bothFilter{vf, ef}
}

// first string is vtype, then pairs after that are props
func Qbv(v ...interface{}) vertexFilter {
	switch len(v) {
	case 0:
		return vertexFilter{VTypeNone, nil}
	case 1, 2:
		return vertexFilter{v[0].(types.VType), nil}
	default:
		vf := vertexFilter{v[0].(types.VType), nil}
		v = v[1:]

		var k string
		var v2 interface{}
		for len(v) > 1 {
			k, v2, v = v[0].(string), v[1], v[2:]
			vf.props = append(vf.props, PropQ{k, v2})
		}

		return vf
	}
}

// first string is etype, then pairs after that are props
func Qbe(v ...interface{}) edgeFilter {
	switch len(v) {
	case 0:
		return edgeFilter{ETypeNone, nil}
	case 1, 2:
		return edgeFilter{v[0].(types.EType), nil}
	default:
		ef := edgeFilter{v[0].(types.EType), nil}
		v = v[1:]

		var k string
		var v2 interface{}
		for len(v) > 1 {
			k, v2, v = v[0].(string), v[1], v[2:]
			ef.props = append(ef.props, PropQ{k, v2})
		}

		return ef
	}
}

// Inspects the indicated vertex's set of out-edges, returning a slice of
// those that match on type and properties. ETypeNone and nil can be passed
// for the last two parameters respectively, in which case the filters will
// be bypassed.
func (g *coreGraph) OutWith(egoId int, ef EFilter) (es []StandardEdge) {
	return g.arcWith(egoId, ef, false)
}

// Inspects the indicated vertex's set of in-edges, returning a slice of
// those that match on type and properties. ETypeNone and nil can be passed
// for the last two parameters respectively, in which case the filters will
// be bypassed.
func (g *coreGraph) InWith(egoId int, ef EFilter) (es []StandardEdge) {
	return g.arcWith(egoId, ef, true)
}

func (g *coreGraph) arcWith(egoId int, ef EFilter, in bool) (es []StandardEdge) {
	etype, props := ef.EType(), ef.EProps()
	vt, err := g.Get(egoId)
	if err != nil {
		// vertex doesn't exist
		return
	}

	var fef func(k string, v ps.Any)
	// TODO specialize the func for zero-cases
	fef = func(k string, v ps.Any) {
		edge := v.(StandardEdge)
		if etype != ETypeNone && etype != edge.EType {
			// etype doesn't match
			return
		}

		for _, p := range props {
			eprop, exists := edge.Props.Lookup(p.K)
			if !exists {
				return
			}

			deprop := eprop.(Property)
			switch tv := deprop.Value.(type) {
			default:
				if tv != p.V {
					return
				}
			case []byte:
				cmptv, ok := p.V.([]byte)
				if !ok || !bytes.Equal(tv, cmptv) {
					return
				}
			}
		}

		es = append(es, edge)
	}

	if in {
		vt.ie.ForEach(fef)
	} else {
		vt.oe.ForEach(fef)
	}

	return
}

// Return a slice of vtTuples that are successors of the given vid, constraining the list
// to those that are connected by edges that pass the EdgeFilter, and the successor
// vertices pass the VertexFilter.
func (g *coreGraph) SuccessorsWith(egoId int, vef VEFilter) (vts []VertexTuple) {
	return g.adjacentWith(egoId, vef, false)
}

// Return a slice of vtTuples that are predecessors of the given vid, constraining the list
// to those that are connected by edges that pass the EdgeFilter, and the predecessor
// vertices pass the VertexFilter.
func (g *coreGraph) PredecessorsWith(egoId int, vef VEFilter) (vts []VertexTuple) {
	return g.adjacentWith(egoId, vef, true)
}

func (g *coreGraph) adjacentWith(egoId int, vef VEFilter, in bool) (vts []VertexTuple) {
	etype, eprops := vef.EType(), vef.EProps()
	vtype, vprops := vef.VType(), vef.VProps()
	vt, err := g.Get(egoId)
	if err != nil {
		// vertex doesn't exist
		return
	}

	// Allow some quick parallelism by continuing to search edges while loading vertices
	// TODO performance test this to see if it's at all worth it
	vidchan := make(chan int, 10)

	var feef func(k string, v ps.Any)
	// TODO specialize the func for zero-cases
	feef = func(k string, v ps.Any) {
		edge := v.(StandardEdge)
		if etype != ETypeNone && etype != edge.EType {
			// etype doesn't match
			return
		}

		for _, p := range eprops {
			eprop, exists := edge.Props.Lookup(p.K)
			if !exists {
				return
			}

			deprop := eprop.(Property)
			switch tv := deprop.Value.(type) {
			default:
				if tv != p.V {
					return
				}
			case []byte:
				cmptv, ok := p.V.([]byte)
				if !ok || !bytes.Equal(tv, cmptv) {
					return
				}
			}
		}

		if in {
			vidchan <- edge.Source
		} else {
			vidchan <- edge.Target
		}
	}

	go func() {
		if in {
			vt.ie.ForEach(feef)
		} else {
			vt.oe.ForEach(feef)
		}
		close(vidchan)
	}()

	// Keep track of the vertices we've collected, for deduping
	visited := make(map[int]struct{})
VertexInspector:
	for vid := range vidchan {
		if _, seenit := visited[vid]; seenit {
			// already visited this vertex; go back to waiting on the chan
			continue
		}
		// mark this vertex as black
		visited[vid] = struct{}{}

		adjvt, err := g.Get(vid)
		if err != nil {
			log.WithFields(log.Fields{
				"system": "engine",
				"err":    err,
			}).Error("Attempted to get nonexistent vid during traversal - should be impossible")
			continue
		}

		// FIXME can't rely on Typ() method here, need to store it
		if vtype != VTypeNone && vtype != adjvt.v.Typ() {
			continue
		}

		for _, p := range vprops {
			vprop, exists := adjvt.v.Props().Lookup(p.K)
			if !exists {
				continue VertexInspector
			}

			dvprop := vprop.(Property)
			switch tv := dvprop.Value.(type) {
			default:
				if tv != p.V {
					continue VertexInspector
				}
			case []byte:
				cmptv, ok := p.V.([]byte)
				if !ok || !bytes.Equal(tv, cmptv) {
					continue VertexInspector
				}
			}
		}

		vts = append(vts, adjvt)
	}

	return
}

// Returns a slice of vertices matching the filter conditions.
//
// The first parameter, VType, filters on vertex type; passing VTypeNone
// will bypass the filter.
//
// The second parameter allows filtering on a k/v property pair.
func (g *coreGraph) VerticesWith(vf VFilter) (vs []VertexTuple) {
	vtype, props := vf.VType(), vf.VProps()
	g.vtuples.ForEach(func(_ string, val ps.Any) {
		vt := val.(VertexTuple)
		if vtype != VTypeNone && vt.v.Typ() != vtype {
			return
		}

		for _, p := range props {
			vprop, exists := vt.v.Props().Lookup(p.K)
			if !exists {
				return
			}

			dvprop := vprop.(Property)
			switch tv := dvprop.Value.(type) {
			default:
				if tv != p.V {
					return
				}
			case []byte:
				cmptv, ok := p.V.([]byte)
				if !ok || !bytes.Equal(tv, cmptv) {
					return
				}
			}
		}

		vs = append(vs, vt)
	})

	return vs
}

func FindEnvironment(g CoreGraph, props ps.Map) (envid int, success bool) {
	rv := g.VerticesWith(Qbv(VType("environment")))
	for _, vt := range rv {
		if matchEnvLink(props, vt.v.Props()) {
			return vt.id, true
		}
	}

	return
}

func FindDataset(g CoreGraph, envid int, name []string) (id int, success bool) {
	// first time through use the parent type
	vtype := VType("parent-dataset")

	var n string
	for len(name) > 0 {
		n, name = name[0], name[1:]
		rv := g.PredecessorsWith(envid, Qbv(vtype, "name", n))
		vtype = "dataset"

		if len(rv) != 1 {
			return 0, false
		}

		id = rv[0].id
	}

	return id, true
}
