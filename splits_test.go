package main

import (
	"reflect"
	"testing"
)

func TestParseSplits(t *testing.T) {
	tests := []struct {
		in      string
		want    []float64
		wantErr bool
	}{
		{"", []float64{}, false},
		{"   ", []float64{}, false},
		{"30,66,92", []float64{30, 66, 92}, false},
		{" 30, 66 ,92 ", []float64{30, 66, 92}, false},
		{"30 66", []float64{30, 66}, false},
		{"33.5,66.25", []float64{33.5, 66.25}, false},
		{"50%", []float64{50}, false},
		{"30,,66", []float64{30, 66}, false},
		{"0", nil, true},
		{"100", nil, true},
		{"-5", nil, true},
		{"120", nil, true},
		{"66,30", nil, true},
		{"30,30", nil, true},
		{"abc", nil, true},
		{"NaN", nil, true},
		{"Inf", nil, true},
	}
	for _, tt := range tests {
		got, err := ParseSplits(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseSplits(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
			t.Errorf("ParseSplits(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestBuildPanelsCols(t *testing.T) {
	ps, err := BuildPanels(3000, 1000, nil, [][]float64{{30, 66, 92}})
	if err != nil {
		t.Fatal(err)
	}
	want := []struct{ x, w int }{{0, 900}, {900, 1080}, {1980, 780}, {2760, 240}}
	if len(ps) != len(want) {
		t.Fatalf("got %d panels, want %d", len(ps), len(want))
	}
	for i, p := range ps {
		if p.X != want[i].x || p.W != want[i].w || p.Y != 0 || p.H != 1000 || p.Index != i || p.Col != i {
			t.Errorf("panel %d = %+v", i, p)
		}
	}
	if ps[0].PW != 0.3 || ps[3].PX != 0.92 || ps[0].PH != 1 {
		t.Errorf("fractions wrong: %+v", ps)
	}
}

func TestBuildPanelsRoundingCoversImage(t *testing.T) {
	ps, err := BuildPanels(1001, 777, []float64{33.3}, [][]float64{{33.3, 66.6}, {50}})
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 5 {
		t.Fatalf("got %d panels", len(ps))
	}
	for _, row := range [][]Panel{ps[:3], ps[3:]} {
		last := row[len(row)-1]
		if last.X+last.W != 1001 {
			t.Errorf("row doesn't reach right edge: %+v", last)
		}
		for i := 1; i < len(row); i++ {
			if row[i].X != row[i-1].X+row[i-1].W {
				t.Errorf("gap between panels: %+v %+v", row[i-1], row[i])
			}
		}
	}
	if ps[3].Y != ps[0].H || ps[3].Y+ps[3].H != 777 {
		t.Errorf("rows don't tile height: %+v %+v", ps[0], ps[3])
	}
	if ps[4].Row != 1 || ps[4].Col != 1 || ps[4].Index != 4 {
		t.Errorf("bad indices: %+v", ps[4])
	}
}

func TestBuildPanelsRows(t *testing.T) {
	ps, err := BuildPanels(800, 2400, []float64{25, 50}, [][]float64{{}, {}, {}})
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 3 || ps[1].Y != 600 || ps[1].H != 600 || ps[2].H != 1200 || ps[2].W != 800 {
		t.Errorf("got %+v", ps)
	}
}

func TestBuildPanelsErrors(t *testing.T) {
	if _, err := BuildPanels(100, 100, []float64{50}, [][]float64{{}}); err == nil {
		t.Error("expected mismatch error")
	}
	if _, err := BuildPanels(10, 10, nil, [][]float64{{50, 51}}); err == nil {
		t.Error("expected too-close error")
	}
}

func TestLayoutNext(t *testing.T) {
	if LayoutCols.Next() != LayoutRows || LayoutRows.Next() != LayoutGrid || LayoutGrid.Next() != LayoutCols {
		t.Error("bad cycle")
	}
}
