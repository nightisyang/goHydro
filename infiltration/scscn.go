package infiltration

import "math"

// Scscn returns cumulative post-abstraction storm retention in metres using
// the NRCS (formerly SCS) curve-number method. p is cumulative rainfall in
// metres remaining after the caller accounts for initial abstraction; cn is
// the normal-condition curve number. amc selects dry (1), normal (2), or wet (3)
// antecedent soil moisture. Inputs are not validated.
//
// Runoff is p minus the returned depth. The return excludes initial abstraction
// and is not an infiltration rate. The caller defines storm boundaries and
// obtains interval depths by differencing consecutive cumulative results.
func Scscn(p, cn float64, amc int) float64 {
	if cn >= 100. {
		return 0.
	}
	if p == 0 {
		return 0
	}
	// Adjust CN based on antecedent soil conditions:
	// AMC conversion factors from: Hawkins, R.H., A.T. Hjelmfelt, A.W. Zevenbergen, 1985. Runoff Probability, Storm Depth, and Curve Numbers. Journal of the Irrigation and Drainage Division, ASCE 111(4). pp.330-340.
	if amc == 1 { // dry AMCI
		cn /= 2.281 - 0.01281*cn
	} else if amc == 3 { // wet AMCIII
		cn /= .427 + 0.00573*cn
	}
	if cn == 0 {
		return p
	}
	// scn := 1000. / cn - 10. // inches
	scn := 25.4/cn - 0.254             // m
	return p - math.Pow(p, 2.)/(p+scn) // post-abstraction retention
}
