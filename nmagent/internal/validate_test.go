package internal

import "testing"

func TestValidate(t *testing.T) {
	validateTests := []struct {
		name          string
		sub           interface{}
		shouldBeValid bool
		shouldPanic   bool
	}{
		{
			"empty",
			struct{}{},
			true,
			false,
		},
		{
			"no tags",
			struct {
				Foo string
			}{""},
			true,
			false,
		},
		{
			"presence",
			struct {
				Foo string `validate:"presence"`
			}{"hi"},
			true,
			false,
		},
		{
			"presence empty",
			struct {
				Foo string `validate:"presence"`
			}{},
			false,
			false,
		},
		{
			"required empty slice",
			struct {
				Foo []string `validate:"presence"`
			}{},
			false,
			false,
		},
		{
			"not a struct",
			42,
			false,
			true,
		},
		{
			"slice",
			[]interface{}{},
			false,
			true,
		},
		{
			"map",
			map[string]interface{}{},
			false,
			true,
		},
	}

	for _, test := range validateTests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if err := recover(); err != nil && !test.shouldPanic {
					t.Fatal("unexpected panic received: err:", err)
				} else if err == nil && test.shouldPanic {
					t.Fatal("expected panic but received none")
				}
			}()
			t.Parallel()

			err := Validate(test.sub)
			if err != nil && test.shouldBeValid {
				t.Fatal("unexpected error validating: err:", err)
			}

			if err == nil && !test.shouldBeValid {
				t.Fatal("expected subject to be invalid but wasn't")
			}
		})
	}
}
