package engine

import "testing"

func TestNormalizeSubject(t *testing.T) {
	cases := map[string]string{
		"  Технології  DevOps  ":       "технології devops",
		"Основи розробки трансляторів": "основи розробки трансляторів",
		"Об’єктно-орієнтоване програмування": "об'єктно-орієнтоване програмування",
		"Бази даних.":                  "бази даних",
	}
	for in, want := range cases {
		got := NormalizeSubject(in)
		if got != want {
			t.Errorf("NormalizeSubject(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeTag(t *testing.T) {
	cases := map[string]string{
		"Лекція":     "lec",
		"Лек.":       "lec",
		"Практика":   "prac",
		"Семінар":    "prac",
		"Лабораторна": "lab",
		"Consultation": "",
	}
	for in, want := range cases {
		got := NormalizeTag(in)
		if got != want {
			t.Errorf("NormalizeTag(%q) = %q, want %q", in, got, want)
		}
	}
}
