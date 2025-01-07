package progress

import (
	"sync"
	"time"
	"wpm/pkg/streams"

	"github.com/briandowns/spinner"
)

type Progress struct {
	ProgressIndicatorEnabled bool
	progressIndicator        *spinner.Spinner
	progressIndicatorMu      sync.Mutex
}

func (p *Progress) StartProgressIndicator(out *streams.Out) {
	p.StartProgressIndicatorWithLabel("", out)
}

func (p *Progress) StartProgressIndicatorWithLabel(label string, s *streams.Out) {
	if !p.ProgressIndicatorEnabled {
		return
	}

	p.progressIndicatorMu.Lock()
	defer p.progressIndicatorMu.Unlock()

	if p.progressIndicator != nil {
		if label == "" {
			p.progressIndicator.Prefix = ""
		} else {
			p.progressIndicator.Prefix = label + " "
		}
		return
	}

	// https://github.com/briandowns/spinner#available-character-sets
	dotStyle := spinner.CharSets[11]
	sp := spinner.New(dotStyle, 120*time.Millisecond, spinner.WithWriter(s), spinner.WithColor("fgCyan"))
	if label != "" {
		sp.Prefix = label + " "
	}

	sp.Start()
	p.progressIndicator = sp
}

func (p *Progress) StopProgressIndicator() {
	p.progressIndicatorMu.Lock()
	defer p.progressIndicatorMu.Unlock()
	if p.progressIndicator == nil {
		return
	}
	p.progressIndicator.Stop()
	p.progressIndicator = nil
}

func (p *Progress) RunWithProgress(label string, run func() error, out *streams.Out) error {
	p.StartProgressIndicatorWithLabel(label, out)
	defer p.StopProgressIndicator()

	return run()
}
