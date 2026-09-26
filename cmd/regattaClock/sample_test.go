package main

import (
	"testing"
	"time"

	"github.com/comagnaw/regattaClock/internal/sample"
)

func TestSampleRequested(t *testing.T) {
	day := time.Date(2026, time.October, 3, 0, 0, 0, 0, time.Local)
	for _, tc := range []struct {
		name    string
		args    []string
		ok      bool
		wantErr bool
		want    sample.Options
	}{
		{name: "absent", args: nil},
		{name: "fyne flags only", args: []string{"-fyne-debug"}},
		{name: "dir only", args: []string{"-dev-sample-regatta", "/tmp/s"}, ok: true,
			want: sample.Options{Dir: "/tmp/s", Unraced: sample.DefaultUnraced}},
		{name: "equals and double dash", args: []string{"--dev-sample-regatta=/tmp/s", "-dev-sample-unraced=2"}, ok: true,
			want: sample.Options{Dir: "/tmp/s", Unraced: 2}},
		{name: "all three", args: []string{"-dev-sample-date", "2026-10-03", "-dev-sample-unraced", "0", "-dev-sample-regatta", "D:\\share\\s"}, ok: true,
			want: sample.Options{Dir: "D:\\share\\s", Unraced: 0, Date: day}},
		{name: "beside other flags", args: []string{"-fyne-debug", "-dev-sample-regatta", "s"}, ok: true,
			want: sample.Options{Dir: "s", Unraced: sample.DefaultUnraced}},
		{name: "missing dir value", args: []string{"-dev-sample-regatta"}, wantErr: true},
		{name: "empty dir", args: []string{"-dev-sample-regatta="}, wantErr: true},
		{name: "bad unraced", args: []string{"-dev-sample-regatta", "s", "-dev-sample-unraced", "-1"}, wantErr: true},
		{name: "bad date", args: []string{"-dev-sample-regatta", "s", "-dev-sample-date", "10/03/2026"}, wantErr: true},
		{name: "date without dir", args: []string{"-dev-sample-date", "2026-10-03"}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok, err := sampleRequested(tc.args)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if !ok {
				return
			}
			if got.Dir != tc.want.Dir || got.Unraced != tc.want.Unraced || !got.Date.Equal(tc.want.Date) {
				t.Errorf("opts = %+v, want %+v", got, tc.want)
			}
		})
	}
}
