package represent

import (
	"github.com/mndrix/ps"
	"github.com/sdboyer/pipeviz/interpret"
)

// Attempts to resolve an EdgeSpec into a real edge. This process has two steps:
//
// 1. Finding the target node.
// 2. Seeing if this edge already exists between source and target.
//
// It is the responsibility of the edge spec's type handler to determine what "if an edge
// already exists" means, as well as whether to overwrite/merge or duplicate the edge in such a case.
func Resolve(g *CoreGraph, mid int, src vtTuple, d EdgeSpec) (StandardEdge, bool) {
	switch es := d.(type) {
	case interpret.EnvLink:
		return resolveEnvLink(g, mid, src, es)
	case interpret.DataLink:
		return resolveDataLink(g, mid, src, es)
	case SpecCommit:
		return resolveSpecCommit(g, src, es)
	case SpecLocalLogic:
		return resolveSpecLocalLogic(g, src, es)
	}

	return StandardEdge{}, false
}

func resolveEnvLink(g *CoreGraph, mid int, src vtTuple, es interpret.EnvLink) (e StandardEdge, success bool) {
	e = StandardEdge{
		Source: src.id,
		Props:  ps.NewMap(),
		Label:  "envlink",
	}

	// First, check if this vertex already *has* an outbound envlink; semantics dictate there can be only one.
	src.oe.ForEach(func(_ string, val ps.Any) {
		edge := val.(StandardEdge)
		if edge.Label == "envlink" {
			success = true
			// FIXME need a way to cut out early
			e = edge
		}
	})

	// Whether we find a match or not, have to merge in the EnvLink
	if es.Address.Hostname != "" {
		e.Props = e.Props.Set("hostname", Property{MsgSrc: mid, Value: es.Address.Hostname})
	}
	if es.Address.Ipv4 != "" {
		e.Props = e.Props.Set("ipv4", Property{MsgSrc: mid, Value: es.Address.Ipv4})
	}
	if es.Address.Ipv6 != "" {
		e.Props = e.Props.Set("ipv6", Property{MsgSrc: mid, Value: es.Address.Ipv6})
	}
	if es.Nick != "" {
		e.Props = e.Props.Set("nick", Property{MsgSrc: mid, Value: es.Nick})
	}

	// If we already found the matching edge, bail out now
	if success {
		return e, true
	}

	g.Vertices(func(vtx Vertex, id int) bool {
		if v, ok := vtx.(environmentVertex); !ok {
			return false
		}

		// TODO for now we're just gonna return out the first matching edge
		props := vtx.Props()

		var val interface{}
		var exists bool

		if val, exists := props.Lookup("hostname"); exists && val == es.Address.Hostname {
			success = true
			e.Target = id
			return true
		}
		if val, exists := props.Lookup("ipv4"); exists && val == es.Address.Ipv4 {
			success = true
			e.Target = id
			return true
		}
		if val, exists := props.Lookup("ipv6"); exists && val == es.Address.Ipv6 {
			success = true
			e.Target = id
			return true
		}
		if val, exists := props.Lookup("nick"); exists && val == es.Nick {
			success = true
			e.Target = id
			return true
		}

		return false
	})

	return e, success
}

func resolveDataLink(g *CoreGraph, mid int, src vtTuple, es interpret.DataLink) (e StandardEdge, success bool) {
	e = StandardEdge{
		Source: src.id,
		Props:  ps.NewMap(),
		Label:  "datalink",
	}

	// DataLinks have a 'name' field that is expected to be unique for the source, if present
	if es.Name != "" {
		// TODO 'name' is a traditional unique key; a change in it inherently denotes a new edge. how to handle this?
		// FIXME this approach just always updates the mid, which is weird?
		e.Props = e.Props.Set("name", Property{MsgSrc: mid, Value: es.Name})

		src.oe.ForEach(func(_ string, val ps.Any) {
			edge := val.(StandardEdge)
			if name, exists := edge.Props.Lookup("name"); exists && edge.Label == "datalink" && name == es.Name {
				// FIXME need a way to cut out early
				success = true
				e = edge
			}
		})
	}

	if es.Type != "" {
		e.Props = e.Props.Set("type", Property{MsgSrc: mid, Value: es.Type})
	}
	if es.Subset != "" {
		e.Props = e.Props.Set("subset", Property{MsgSrc: mid, Value: es.Subset})
	}
	if es.Interaction != "" {
		e.Props = e.Props.Set("interaction", Property{MsgSrc: mid, Value: es.Interaction})
	}

	// Special bits: if we have ConnUnix data, eliminate ConnNet data, and vice-versa.
	var isLocal bool
	if es.ConnUnix.Path != "" {
		isLocal = true
		e.Props = e.Props.Set("path", Property{MsgSrc: mid, Value: es.ConnUnix.Path})
		e.Props = e.Props.Delete("hostname")
		e.Props = e.Props.Delete("ipv4")
		e.Props = e.Props.Delete("ipv6")
		e.Props = e.Props.Delete("port")
		e.Props = e.Props.Delete("proto")
	} else {
		e.Props = e.Props.Set("port", Property{MsgSrc: mid, Value: es.ConnNet.Port})
		e.Props = e.Props.Set("proto", Property{MsgSrc: mid, Value: es.ConnNet.Proto})

		// can only be one of hostname, ipv4 or ipv6
		if es.ConnNet.Hostname != "" {
			e.Props = e.Props.Set("hostname", Property{MsgSrc: mid, Value: es.ConnNet.Hostname})
		} else if es.ConnNet.Ipv4 != "" {
			e.Props = e.Props.Set("ipv4", Property{MsgSrc: mid, Value: es.ConnNet.Ipv4})
		} else {
			e.Props = e.Props.Set("ipv6", Property{MsgSrc: mid, Value: es.ConnNet.Ipv6})
		}
	}

	if success {
		return e, true
	}

	// if net, must scan. if local, a bit easier.

	if isLocal {
		envid, exists := findEnv(g, src)
		if !exists {
			// this is would be a pretty weird case
			return e, false
		}

		envvt, _ := g.Get(envid)
		envvt.ie.ForEach(func(_ string, val ps.Any) {
			e2 := val.(StandardEdge)
			if e2.Label == "envlink" {
				// FIXME really, really not lovely to have to scan through all these like this
				vt, err := g.Get(e2.Target)
				if err != nil {
					// err means not found; skip
					return
				}

				if vt.v.Typ() == "process" {
					// TODO ugh this is where we need multi-vertex returns from splitters
				}
			}
		})
	} else {
		g.Vertices(func(vtx Vertex, id int) bool {
			return false
		})
	}

	return e, false
}

//func resolveSpecCommit(g *CoreGraph, src vtTuple, e SpecCommit) (StandardEdge, bool) {
//g.Vertices(func(vtx Vertex, id int) bool {})
//}

//func resolveSpecLocalLogic(g *CoreGraph, src vtTuple, e SpecLocalLogic) (StandardEdge, bool) {
//g.Vertices(func(vtx Vertex, id int) bool {})
//}

// Searches the given vertex's out-edges to find its environment's vertex id.
func findEnv(g *CoreGraph, vt vtTuple) (id int, success bool) {
	vt.oe.ForEach(func(_ string, val ps.Any) {
		edge := val.(StandardEdge)
		if edge.Label == "envlink" {
			success = true
			// FIXME need a way to cut out early
			id = edge.Target
		}
	})

	return
}

//func findByListener(g *CoreGraph, vt vtTuple) (vtTuple, bool) {

//}
