package cmd

type BreakpointConfig struct {
	Executor   string                 `json:"executor"`
	Stages     []Stage                `json:"stages"`
	Thresholds map[string][]Threshold `json:"thresholds"`
}

type Stage struct {
	Duration string `json:"duration"`
	Target   int    `json:"target"`
}

type Threshold struct {
	Threshold      string `json:"threshold"`
	AbortOnFail    bool   `json:"abortOnFail"`
	DelayAbortEval string `json:"delayAbortEval"`
}
