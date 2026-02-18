package utils

import "fmt"

func Plural(n int64, forms [3]string) string {
	nAbs := n
	if nAbs < 0 {
		nAbs = -nAbs
	}

	var form string
	if nAbs%10 == 1 && nAbs%100 != 11 {
		form = forms[0]
	} else if nAbs%10 >= 2 && nAbs%10 <= 4 && (nAbs%100 < 10 || nAbs%100 >= 20) {
		form = forms[1]
	} else {
		form = forms[2]
	}

	return fmt.Sprintf("%d %s", n, form)
}
