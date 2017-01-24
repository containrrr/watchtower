package container

func sliceEqual(s1, s2 []string) bool {
	if len(s1) != len(s2) {
		return false
	}

	for i := range s1 {
		if s1[i] != s2[i] {
			return false
		}
	}

	return true
}

func sliceSubtract(a1, a2 []string) []string {
	a := []string{}

	for _, e1 := range a1 {
		found := false

		for _, e2 := range a2 {
			if e1 == e2 {
				found = true
				break
			}
		}

		if !found {
			a = append(a, e1)
		}
	}

	return a
}

func stringMapSubtract(m1, m2 map[string]string) map[string]string {
	m := map[string]string{}

	for k1, v1 := range m1 {
		if v2, ok := m2[k1]; ok {
			if v2 != v1 {
				m[k1] = v1
			}
		} else {
			m[k1] = v1
		}
	}

	return m
}

func structMapSubtract(m1, m2 map[string]struct{}) map[string]struct{} {
	m := map[string]struct{}{}

	for k1, v1 := range m1 {
		if _, ok := m2[k1]; !ok {
			m[k1] = v1
		}
	}

	return m
}
