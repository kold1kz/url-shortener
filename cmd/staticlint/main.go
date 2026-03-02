package main

import (
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"

	// стандартные анализаторы x/tools/go/analysis/passes
	"golang.org/x/tools/go/analysis/passes/assign"
	"golang.org/x/tools/go/analysis/passes/atomic"
	"golang.org/x/tools/go/analysis/passes/bools"
	"golang.org/x/tools/go/analysis/passes/buildtag"
	"golang.org/x/tools/go/analysis/passes/cgocall"
	"golang.org/x/tools/go/analysis/passes/composite"
	"golang.org/x/tools/go/analysis/passes/copylock"
	"golang.org/x/tools/go/analysis/passes/errorsas"
	"golang.org/x/tools/go/analysis/passes/httpresponse"
	"golang.org/x/tools/go/analysis/passes/loopclosure"
	"golang.org/x/tools/go/analysis/passes/lostcancel"
	"golang.org/x/tools/go/analysis/passes/nilfunc"
	"golang.org/x/tools/go/analysis/passes/printf"
	"golang.org/x/tools/go/analysis/passes/shadow"
	"golang.org/x/tools/go/analysis/passes/shift"
	"golang.org/x/tools/go/analysis/passes/stdmethods"
	"golang.org/x/tools/go/analysis/passes/structtag"
	"golang.org/x/tools/go/analysis/passes/tests"
	"golang.org/x/tools/go/analysis/passes/unmarshal"
	"golang.org/x/tools/go/analysis/passes/unreachable"
	"golang.org/x/tools/go/analysis/passes/unsafeptr"
	"golang.org/x/tools/go/analysis/passes/unusedresult"

	// staticcheck классы
	"honnef.co/go/tools/simple"
	"honnef.co/go/tools/staticcheck"
	"honnef.co/go/tools/stylecheck"

	// “публичные анализаторы”
	"github.com/go-critic/go-critic/checkers/analyzer"
	"github.com/timakin/bodyclose/passes/bodyclose"

	// мой анализатор
	"url-shortener/cmd/staticlint/noosexit"
)

func main() {
	var analyzers []*analysis.Analyzer

	// 1) стандартные анализаторы passes
	analyzers = append(analyzers,
		assign.Analyzer,
		atomic.Analyzer,
		bools.Analyzer,
		buildtag.Analyzer,
		cgocall.Analyzer,
		composite.Analyzer,
		copylock.Analyzer,
		errorsas.Analyzer,
		httpresponse.Analyzer,
		loopclosure.Analyzer,
		lostcancel.Analyzer,
		nilfunc.Analyzer,
		printf.Analyzer,
		shadow.Analyzer,
		shift.Analyzer,
		stdmethods.Analyzer,
		structtag.Analyzer,
		tests.Analyzer,
		unmarshal.Analyzer,
		unreachable.Analyzer,
		unsafeptr.Analyzer,
		unusedresult.Analyzer,
	)

	// 2) все анализаторы класса SA из staticcheck
	for _, a := range staticcheck.Analyzers {
		if len(a.Analyzer.Name) >= 2 && a.Analyzer.Name[:2] == "SA" {
			analyzers = append(analyzers, a.Analyzer)
		}
	}

	// анализатор других классов (S / ST / QF)
	if len(simple.Analyzers) > 0 {
		analyzers = append(analyzers, simple.Analyzers[0].Analyzer) // Sxxxx
	}
	if len(stylecheck.Analyzers) > 0 {
		analyzers = append(analyzers, stylecheck.Analyzers[0].Analyzer) // STxxxx
	}

	// публичных анализаторов
	analyzers = append(analyzers,
		analyzer.Analyzer,  // go-critic
		bodyclose.Analyzer, // bodyclose
	)

	// 5) собственный анализатор
	analyzers = append(analyzers, noosexit.Analyzer)

	multichecker.Main(analyzers...)
}
