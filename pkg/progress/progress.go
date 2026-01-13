package progress

import (
	"io"
	"sync"
	"time"
	"wpm/pkg/unsafeconv"

	"github.com/briandowns/spinner"
)

type Progress struct {
	ProgressColorEnabled     bool
	ProgressIndicatorEnabled bool
	progressIndicator        *spinner.Spinner
	progressIndicatorMu      sync.Mutex
}

func (p *Progress) StartProgressIndicator(out io.Writer) {
	p.StartProgressIndicatorWithLabel("", out)
}

func (p *Progress) StartProgressIndicatorWithLabel(label string, s io.Writer) {
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
	var sp *spinner.Spinner
	if p.ProgressColorEnabled {
		dotStyle := spinner.CharSets[11]
		sp = spinner.New(dotStyle, 120*time.Millisecond, spinner.WithWriter(s), spinner.WithColor("fgCyan"))
	} else {
		dotStyle := spinner.CharSets[14]
		sp = spinner.New(dotStyle, 120*time.Millisecond, spinner.WithWriter(s))
	}

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

func (p *Progress) RunWithProgress(label string, run func() error, out io.Writer) error {
	p.StartProgressIndicatorWithLabel(label, out)
	defer p.StopProgressIndicator()

	return run()
}

func (p *Progress) Stream(out io.Writer, text string) {
	p.progressIndicatorMu.Lock()
	defer p.progressIndicatorMu.Unlock()

	if p.progressIndicator != nil && p.progressIndicator.Active() {
		p.progressIndicator.Stop()
	}

	out.Write(unsafeconv.UnsafeStringToBytes("\r" + text + "\033[K"))
}
