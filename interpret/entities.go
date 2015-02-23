package interpret

type Environment struct {
	Address     Address      `json:"address"`
	Os          string       `json:"os"`
	Provider    string       `json:"provider"`
	Type        string       `json:"type"`
	Nickname    string       `json:"nickname"`
	LogicStates []LogicState `json:"logic-states"`
	Datasets    []Dataset    `json:"datasets"`
	Processes   []Process    `json:"processes"`
}

type EnvLink struct {
	Address Address `json:"address"`
	Nick    string  `json:"nick"`
}

type Address struct {
	Hostname string `json:"hostname"`
	Ipv4     string `json:"ipv4"`
	Ipv6     string `json:"ipv6"`
}

type LogicState struct {
	Datasets    []DataLink `json:"datasets"`
	Environment EnvLink    `json:"environment"`
	ID          struct {
		Commit     string `json:"commit"`
		Repository string `json:"repository"`
		Version    string `json:"version"`
		Semver     string `json:"semver"`
	} `json:"id"`
	Lgroup string `json:"lgroup"`
	Nick   string `json:"nick"`
	Path   string `json:"path"`
	Type   string `json:"type"`
}

type DataLink struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Subset      string `json:"subset"`
	Interaction string `json:"interaction"`
	Path        string `json:"path"`
	Rel         struct {
		Hostname string `json:"hostname"`
		Port     int    `json:"port"`
		Proto    string `json:"proto"`
		Type     string `json:"type"`
		Path     string `json:"path"`
	} `json:"rel"`
}

type CommitMeta struct {
	Sha1      []byte   `json:"sha1"`
	Tags      []string `json:"tags"`
	TestState string   `json:"testState"`
}

type Commit struct {
	Author  string   `json:"author"`
	Date    string   `json:"date"`
	Parents [][]byte `json:"parents"`
	Sha1    []byte   `json:"sha1"`
	Subject string   `json:"subject"`
}

type Dataset struct {
	Environment EnvLink `json:"environment"`
	Name        string  `json:"name"`
	Subsets     []struct {
		Name       string `json:"name"`
		CreateTime string `json:"create-time"`
		Genesis    struct {
			Address  Address  `json:"address"`
			Dataset  []string `json:"dataset"`
			SnapTime string   `json:"snap-time"`
		} `json:"genesis"`
	} `json:"subsets"`
}

type Process struct {
	Pid         int     `json:"pid"`
	Cwd         string  `json:"cwd"`
	Environment EnvLink `json:"environment"`
	Group       string  `json:"group"`
	Listen      []struct {
		Port  int      `json:"port"`
		Proto []string `json:"proto"`
		Type  string   `json:"type"`
		Path  string   `json:"path"`
	} `json:"listen"`
	LogicStates []string `json:"logic-states"`
	User        string   `json:"user"`
}
