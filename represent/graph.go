package represent

import "github.com/sdboyer/pipeviz/interpret"

// the main graph construct
type CoreGraph struct {
	vlist   map[int]Vertex
	vserial int
}

type Vertex struct {
	props []Property
}

func (v Vertex) Merge(iv Vertex) Vertex {
	return iv
}

type Property struct {
	MsgSrc int
	Key    string
	Value  interface{}
}

type rootedEdgeSpecTuple struct {
	vid int
	es  EdgeSpecs
}

type edgeSpecSet []rootedEdgeSpecTuple

func (ess edgeSpecSet) EdgeCount() (i int) {
	for _, tuple := range ess {
		i = i + len(tuple.es)
	}
	return
}

// some kind of structure representing the delta introduced by a message
type Delta struct{}

// the method to merge a message into the graph
func (g *CoreGraph) Merge(msg interpret.Message) Delta {
	var ess edgeSpecSet

	// Process incoming elements from the message
	msg.Each(func(d interface{}) {
		// Split each input element into vertex and edge specs
		// TODO errs
		vertex, edges, _ := Split(d, msg.Id)
		// Ensure the vertex is present
		vid := g.ensureVertex(vertex)

		// Collect edge specs for later processing
		ess = append(ess, rootedEdgeSpecTuple{
			vid: vid,
			es:  edges,
		})
	})

	// All vertices processed. now, process edges in passes, ensuring that each
	// pass diminishes the number of remaining edges. If it doesn't, the remaining
	// edges need to be attached to null-vertices of the appropriate type.
	//
	// This is a little wasteful, but it's the simplest way to let any possible
	// dependencies between edges work themselves out. It has provably incorrect
	// cases, however, and will need to be replaced.
	var ec, lec int
	for ec = ess.EdgeCount(); ec != lec; ec = ess.EdgeCount() {
		lec = ec
		for _, tuple := range ess {
			for k, spec := range tuple.es {
				edge, success := Resolve(g, spec)
				if success {
					g.ensureEdge(edge)
					tuple.es = append(tuple.es[:k], tuple.es[k+1:]...)
				}
			}
		}
	}

	return Delta{}
}

// Ensures the vertex is present. Merges according to type-specific logic if
// it is present, otherwise adds the vertex.
//
// Either way, return value is the vid for the vertex.
func (g *CoreGraph) ensureVertex(vertex Vertex) (vid int) {
	vid = g.Find(vertex)

	if vid != 0 {
		v2, _ := g.Get(vid)
		g.vlist[vid] = v2.Merge(vertex)
	} else {
		g.vserial++
		g.vlist[g.vserial] = vertex
		vid = g.vserial
	}

	return vid
}

// TODO add edge to the structure. blah blah handwave blah blah
func (g *CoreGraph) ensureEdge(e StandardEdge) {

}

// the func we eventually aim to fulfill, replacing Merge for integrating messages
//func (g CoreGraph) Cons(interpret.Message) CoreGraph, Delta, error {}

// Searches for an instance of the vertex within the graph. If found,
// returns the vertex id, otherwise returns 0.
//
// TODO this is really a querying method. needs to be replaced by that whole subsystem
func (g *CoreGraph) Find(vertex Vertex) int {
	// FIXME so very hilariously O(n)

	var chk interpret.Identifier
	for _, idf := range interpret.Identifiers {
		if idf.CanIdentify(vertex) {
			chk = idf
		}
	}

	// we hit this case iff there's an object type our identifiers can't work
	// with. which should, eventually, be structurally impossible by this point
	if chk == nil {
		return 0
	}

	for id, v := range g.vlist {
		if chk.Matches(v, vertex) {
			return id
		}
	}

	return 0
}

func (g *CoreGraph) Get(id int) (Vertex, error) {
	return Vertex{}, nil
}
