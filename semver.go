package main

type SemVer struct {
	major int
	minor int
	patch int
}

func (v *SemVer) LessThan(v2 *SemVer) (res bool) {
	if v.major < v2.major {
		return true
	} else if v.major > v2.major {
		return false
	} else if v.minor < v2.minor {
		return true
	} else if v.minor > v2.minor {
		return false
	} else if v.patch < v2.patch {
		return true
	} else {
		return false
	}
}

func (v *SemVer) Equal(v2 *SemVer) (res bool) {
	if v.major == v2.major &&
		v.minor == v2.minor &&
		v.patch == v2.patch {
		return true
	} else {
		return false
	}
}

func (v *SemVer) GreaterThan(v2 *SemVer) (res bool) {
	if v.major > v2.major {
		return true
	} else if v.major < v2.major {
		return false
	} else if v.minor > v2.minor {
		return true
	} else if v.minor < v2.minor {
		return false
	} else if v.patch > v2.patch {
		return true
	} else {
		return false
	}
}
