package care

// Rating is an ecosystem's grading policy: the weight each feature carries in the
// health score and the worst score a failing critical feature is allowed to leave
// standing. The types are language-agnostic (they mirror the Ecosystem's feature
// slots), so every ecosystem grades through the same shape; only the default values
// differ per ecosystem (a Go module and a Node package can weight dependencies
// differently). An ecosystem supplies its defaults as a value (e.g.
// golang.DefaultRating); a care.yml health block overlays it during config load.
type Rating struct {
	Weights Weights `yaml:"weights"`
	Caps    Caps    `yaml:"caps"`
}

// Weights is the relative importance of each gradable feature in the score, one field
// per Ecosystem feature slot. Weight 0 makes a feature informational (excluded from
// the grade), which is the usual setting for benchmarks: they run and report but
// never move the score.
type Weights struct {
	VersionControl  int `yaml:"version_control"`
	Build           int `yaml:"build"`
	Lint            int `yaml:"lint"`
	Dependencies    int `yaml:"dependencies"`
	Docs            int `yaml:"docs"`
	Tests           int `yaml:"tests"`
	Benchmarks      int `yaml:"benchmarks"`
	Secrets         int `yaml:"secrets"`
	Vulnerabilities int `yaml:"vulnerabilities"`
}

// Caps is the score ceiling a failing critical feature imposes: a committed secret is
// a live exposure (bad regardless of dev state), a reachable vulnerability is real but
// often transitive. A feature with no cap relies on its weight alone. Operators own
// their grade, so caps are tunable; loosening one weakens that signal by choice.
type Caps struct {
	Secrets         int `yaml:"secrets"`
	Vulnerabilities int `yaml:"vulnerabilities"`
}

// Rated is implemented by a check that carries its own grading policy: its weight in
// the health score and an optional score cap it imposes when it fails. The runner
// reads these off the check and stamps them onto the result, so the rating engine
// grades from per-check policy registered at the ecosystem assembly site, not from a
// central feature->weight map.
type Rated interface {
	// Weight is the feature's relative importance in the score; 0 makes it
	// informational (excluded from the grade).
	Weight() int
	// Cap is the worst score this check leaves standing when it fails; ok is false
	// when the check imposes no ceiling.
	Cap() (score int, ok bool)
}

// ratedCheck wraps a typed check with its grading policy. It embeds the Check so the
// whole Check surface forwards unchanged, which keeps the wrapper assignable to the
// same Ecosystem slot, and adds Weight/Cap plus a Profiles forward so a wrapped
// profiled feature (tests, benchmarks) still expands into its per-profile tasks.
type ratedCheck[T Report] struct {
	Check[T]
	weight   int
	capScore int
	hasCap   bool
}

// WithRating binds a grading weight (and an optional score cap via CapAt) to a check
// at registration, so a feature's importance lives at the ecosystem assembly site
// next to the check it grades. The result still fills the same typed Ecosystem slot.
func WithRating[T Report](c Check[T], weight int, opts ...RateOption) Check[T] {
	o := rateOptions{}
	for _, opt := range opts {
		opt(&o)
	}
	return &ratedCheck[T]{Check: c, weight: weight, capScore: o.capScore, hasCap: o.hasCap}
}

// RateOption configures a WithRating call.
type RateOption func(*rateOptions)

// rateOptions accumulates the optional grading settings a WithRating call applies.
type rateOptions struct {
	capScore int
	hasCap   bool
}

// CapAt sets the worst score a check leaves standing when it fails, so a critical
// failure (a committed secret, a reachable vulnerability) cannot hide behind an
// otherwise good average.
func CapAt(score int) RateOption {
	return func(o *rateOptions) { o.capScore, o.hasCap = score, true }
}

var _ Rated = (*ratedCheck[VCReport])(nil)

func (r *ratedCheck[T]) Weight() int { return r.weight }

func (r *ratedCheck[T]) Cap() (int, bool) { return r.capScore, r.hasCap }

// Profiles forwards to the wrapped check when it is profiled, so wrapping a profiled
// feature does not hide its profiles from the task expansion; a non-profiled check
// returns nil and expands into a single task.
func (r *ratedCheck[T]) Profiles() []RunProfile {
	if p, ok := r.Check.(Profiled); ok {
		return p.Profiles()
	}
	return nil
}
