package awsread

type ReadInput struct {
	Service   string
	Operation string
	Params    any
	Region    string
}

type ReadOutput struct {
	Service   string `json:"service"`
	Operation string `json:"operation"`
	Region    string `json:"region"`
	Result    any    `json:"result"`
}
