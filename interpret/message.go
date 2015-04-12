package interpret

import (
	"encoding/json"
	"fmt"
)

type Message struct {
	Id int
	m  *message
}

type message struct {
	Env []Environment `json:"environments"`
	Ls  []LogicState  `json:"logic-states"`
	Dms []DataMetaSet `json:"datasets"`
	Ds  []Dataset
	P   []Process    `json:"processes"`
	C   []Commit     `json:"commits"`
	Cm  []CommitMeta `json:"commit-meta"`
}

func (m *Message) UnmarshalJSON(data []byte) error {
	m.m = new(message)
	err := json.Unmarshal(data, m.m)
	if err != nil {
		// FIXME logging
		fmt.Println(err)
	}

	// TODO separate all of this into pluggable/generated structures
	// first, dump all top-level objects into the graph.
	for _, e := range m.m.Env {
		envlink := EnvLink{Address: Address{}}

		// Create an envlink for any nested items, preferring nick, then hostname, ipv4, ipv6.
		if e.Nickname != "" {
			envlink.Nick = e.Nickname
		} else if e.Address.Hostname != "" {
			envlink.Address.Hostname = e.Address.Hostname
		} else if e.Address.Ipv4 != "" {
			envlink.Address.Ipv4 = e.Address.Ipv4
		} else if e.Address.Ipv6 != "" {
			envlink.Address.Ipv6 = e.Address.Ipv6
		}

		// manage the little environment hierarchy
		for _, ls := range e.LogicStates {
			ls.Environment = envlink
			m.m.Ls = append(m.m.Ls, ls)
		}
		for _, p := range e.Processes {
			p.Environment = envlink
			m.m.P = append(m.m.P, p)
		}
		for _, dms := range e.Datasets {
			dms.Environment = envlink
			m.m.Dms = append(m.m.Dms, dms)

			for _, ds := range dms.Subsets {
				// FIXME this doesn't really work as a good linkage
				ds.Parent = dms.Name
				m.m.Ds = append(m.m.Ds, ds)
			}
		}
	}

	return nil
}

func (m *Message) Each(f func(vertex interface{})) {
	for _, e := range m.m.Env {
		f(e)
	}
	for _, e := range m.m.Ls {
		f(e)
	}
	for _, e := range m.m.Dms {
		f(e)
	}
	for _, e := range m.m.Ds {
		f(e)
	}
	for _, e := range m.m.P {
		f(e)
	}
	for _, e := range m.m.C {
		f(e)
	}
	for _, e := range m.m.Cm {
		f(e)
	}
}

func findEnv(envs []Environment, el EnvLink) (Environment, bool) {
	if el.Nick != "" {
		for _, e := range envs {
			if e.Nickname == el.Nick {
				return e, true
			}
		}
	} else if el.Address.Hostname != "" {
		for _, e := range envs {
			if e.Address.Hostname == el.Address.Hostname {
				return e, true
			}
		}
	} else if el.Address.Ipv4 != "" {
		for _, e := range envs {
			if e.Address.Ipv4 == el.Address.Ipv4 {
				return e, true
			}
		}
	} else if el.Address.Ipv6 != "" {
		for _, e := range envs {
			if e.Address.Ipv6 == el.Address.Ipv6 {
				return e, true
			}
		}
	}

	return Environment{}, false
}
