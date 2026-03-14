package main

import "testing"

func TestContainsWord(t *testing.T) {
	tests := []struct {
		lower, kw string
		want      bool
	}{
		// Short keywords with plurals
		{"eggs", "egg", true},
		{"large eggs", "egg", true},
		{"egg whites", "egg", true},
		{"eggplant", "egg", false}, // "egg" + "p" is not a valid boundary
		// 4-char keywords with boundary
		{"milk", "milk", true},
		{"whole milk", "milk", true},
		{"milks", "milk", true}, // trailing s is OK
		{"milky way bar", "milk", false}, // "milky" — "y" is not a valid boundary
		// Short keyword exact match
		{"oj", "oj", true},
		{"oj tropicana", "oj", true},
		// Longer keywords — substring match
		{"banana", "banana", true},
		{"organic bananas", "banana", true},
		{"orange juice", "orange", true},
		{"chicken breast", "chicken", true},
	}
	for _, tc := range tests {
		got := containsWord(tc.lower, tc.kw)
		if got != tc.want {
			t.Errorf("containsWord(%q, %q) = %v, want %v", tc.lower, tc.kw, got, tc.want)
		}
	}
}

func TestInferItemEmoji(t *testing.T) {
	tests := []struct {
		name, dept, want string
	}{
		// Item-specific should override department
		{"Eggs", "Dairy & Eggs", "🥚"},
		{"Large Eggs", "Dairy & Eggs", "🥚"},
		{"Organic Eggs", "Dairy & Eggs", "🥚"},
		{"Orange", "Produce", "🍊"},
		{"Navel Oranges", "Produce", "🍊"},
		{"Banana", "Produce", "🍌"},
		{"Bananas", "Produce", "🍌"},
		{"Milk", "Dairy & Eggs", "🥛"},
		{"Whole Milk", "Dairy & Eggs", "🥛"},
		// Milky Way should NOT match milk
		{"Milky Way Bar", "Snacks & Candy", "🍫"},
		// Department fallback for unknown items
		{"Some Random Item", "Produce", "🥬"},
		{"Unknown Thing", "Beverages", "🥤"},
		// Chicken
		{"Chicken Breast", "Meat & Seafood", "🍗"},
		// Bread
		{"Whole Wheat Bread", "Bakery & Deli", "🍞"},
	}
	for _, tc := range tests {
		got := inferItemEmoji(tc.name, tc.dept)
		if got != tc.want {
			t.Errorf("inferItemEmoji(%q, %q) = %q, want %q", tc.name, tc.dept, got, tc.want)
		}
	}
}
