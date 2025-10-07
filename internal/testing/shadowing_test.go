package main

import "testing"

func TestShadowing(t *testing.T) {
	{
		var x int = 1
		{
			var x int = 2
			if x != 2 {
				t.FailNow()
			}
		}
		if x != 1 {
			t.FailNow()
		}
	}
}
