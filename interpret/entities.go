package interpret

import (
	"bytes"
	"encoding/hex"
)

type Environment struct {
	Address     Address         `json:"address,omitempty"`
	OS          string          `json:"os,omitempty"`
	Provider    string          `json:"provider,omitempty"`
	Type        string          `json:"type,omitempty"`
	Nick        string          `json:"nick,omitempty"`
	LogicStates []LogicState    `json:"logic-states,omitempty"`
	Datasets    []ParentDataset `json:"datasets,omitempty"`
	Processes   []Process       `json:"processes,omitempty"`
}

type EnvLink struct {
	Address Address `json:"address,omitempty"`
	Nick    string  `json:"nick,omitempty"`
}

type Address struct {
	Hostname string `json:"hostname,omitempty"`
	Ipv4     string `json:"ipv4,omitempty"`
	Ipv6     string `json:"ipv6,omitempty"`
}

type LogicState struct {
	Datasets    []DataLink `json:"datasets,omitempty"`
	Environment EnvLink    `json:"environment,omitempty"`
	ID          struct {
		Commit    Sha1   `json:"-"`
		CommitStr string `json:"commit,omitempty"`
		Version   string `json:"version,omitempty"`
		Semver    string `json:"semver,omitempty"`
	} `json:"id,omitempty"`
	Lgroup string `json:"lgroup,omitempty"`
	Nick   string `json:"nick,omitempty"`
	Path   string `json:"path,omitempty"`
	Type   string `json:"type,omitempty"`
}

type DataLink struct {
	Name        string   `json:"name,omitempty"`
	Type        string   `json:"type,omitempty"`
	Subset      string   `json:"subset,omitempty"`
	Interaction string   `json:"interaction,omitempty"`
	ConnUnix    ConnUnix `json:"connUnix,omitempty"`
	ConnNet     ConnNet  `json:"connNet,omitempty"`
}

type ConnNet struct {
	Hostname string `json:"hostname,omitempty"`
	Ipv4     string `json:"ipv4,omitempty"`
	Ipv6     string `json:"ipv6,omitempty"`
	Port     int    `json:"port,omitempty"`
	Proto    string `json:"proto,omitempty"`
}

type ConnUnix struct {
	Path string `json:"path,omitempty"`
}

type Sha1 [20]byte

func (s Sha1) MarshalJSON() ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 42))

	_, err := buf.WriteString(`"`)
	if err != nil {
		return nil, err
	}

	_, err = buf.WriteString(hex.EncodeToString(s[:]))
	if err != nil {
		return nil, err
	}

	_, err = buf.WriteString(`"`)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// IsEmpty checks to see if the Sha1 is equal to the null Sha1 (20 zero bytes/40 ASCII zeroes)
func (s Sha1) IsEmpty() bool {
	// TODO this may have some odd semantic side effects, as git uses this, the "null sha1", to indicate a nonexistent head. not in any places pipeviz will forseeably interact with it, though...
	return s == [20]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
}

// String converts a sha1 to the standard 40-char lower-case hexadecimal representation.
func (s Sha1) String() string {
	return hex.EncodeToString(s[:])
}

type CommitMeta struct {
	Sha1      Sha1
	Sha1Str   string   `json:"sha1,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	Branches  []string `json:"branches,omitempty"`
	TestState string   `json:"testState,omitempty"`
}

type Commit struct {
	Author     string `json:"author,omitempty"`
	Date       string `json:"date,omitempty"`
	Parents    []Sha1
	ParentsStr []string `json:"parents,omitempty"`
	Sha1       Sha1
	Sha1Str    string `json:"sha1,omitempty"`
	Subject    string `json:"subject,omitempty"`
	Repository string `json:"repository,omitempty"`
}

// FIXME this metaset/set design is not recursive, but it will need to be
type ParentDataset struct {
	Environment EnvLink   `json:"environment,omitempty"`
	Path        string    `json:"path,omitempty"`
	Name        string    `json:"name,omitempty"`
	Subsets     []Dataset `json:"subsets,omitempty"`
}

type Dataset struct {
	Name       string      `json:"name,omitempty"`
	Parent     string      // TODO yechhhh...how do we qualify the hierarchy?
	CreateTime string      `json:"create-time,omitempty"`
	Genesis    DataGenesis `json:"genesis,omitempty"`
}

type DataGenesis interface {
	_dg() // dummy method, avoid propagating the interface
}

// TODO this form implies a snap that only existed while 'in flight' - that is, no
// dump artfiact that exists on disk anywhere. Need to incorporate that case, though.
type DataProvenance struct {
	Address  Address  `json:"address,omitempty"`
	Dataset  []string `json:"dataset,omitempty"`
	SnapTime string   `json:"snap-time,omitempty"`
}

type DataAlpha string

func (d DataAlpha) _dg()      {}
func (d DataProvenance) _dg() {}

type Process struct {
	Pid         int          `json:"pid,omitempty"`
	Cwd         string       `json:"cwd,omitempty"`
	Dataset     string       `json:"dataset,omitempty"`
	Environment EnvLink      `json:"environment,omitempty"`
	Group       string       `json:"group,omitempty"`
	Listen      []ListenAddr `json:"listen,omitempty"`
	LogicStates []string     `json:"logic-states,omitempty"`
	User        string       `json:"user,omitempty"`
}

type ListenAddr struct {
	Port  int      `json:"port,omitempty"`
	Proto []string `json:"proto,omitempty"`
	Type  string   `json:"type,omitempty"`
	Path  string   `json:"path,omitempty"`
}

type YumPkg struct {
	Name       string `json:"name,omitempty"`
	Repository string `json:"repository,omitempty"`
	Version    string `json:"version,omitempty"`
	Epoch      int    `json:"epoch,omitempty"`
	Release    string `json:"release,omitempty"`
	Arch       string `json:"arch,omitempty"`
}
