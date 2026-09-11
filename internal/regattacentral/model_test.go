package regattacentral

import (
	"testing"
	"time"
)

func TestUploadRequestValidate(t *testing.T) {
	tests := []struct {
		name    string
		req     UploadRequest
		assume  bool
		wantErr bool
	}{
		{name: "no results", req: UploadRequest{Lanes: []LaneRecord{{RaceNumber: 1, Lane: 1}}}},
		{
			name: "result with matching lane",
			req: UploadRequest{
				Lanes:   []LaneRecord{{RaceNumber: 1, Lane: 2}},
				Results: []ResultRecord{{RaceNumber: 1, Lane: 2, TimingMilestoneID: MilestoneFinish}},
			},
		},
		{
			name:    "result without matching lane",
			req:     UploadRequest{Results: []ResultRecord{{RaceNumber: 1, Lane: 2}}},
			wantErr: true,
		},
		{
			name:   "result without lane but assumed uploaded",
			req:    UploadRequest{Results: []ResultRecord{{RaceNumber: 1, Lane: 2}}},
			assume: true,
		},
		{
			name: "one of several results missing its lane",
			req: UploadRequest{
				Lanes: []LaneRecord{{RaceNumber: 5, Lane: 1}},
				Results: []ResultRecord{
					{RaceNumber: 5, Lane: 1},
					{RaceNumber: 5, Lane: 2},
				},
			},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.req.Validate(tc.assume)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Validate = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestSetRaceStatusUpdatesInPlace(t *testing.T) {
	var u UploadRequest
	u.SetRaceStatus(3, StatusPreDraw)
	u.SetRaceStatus(4, StatusDraw)
	u.SetRaceStatus(3, StatusRacing) // update, not append

	if len(u.Races) != 2 {
		t.Fatalf("races = %d, want 2", len(u.Races))
	}
	if u.Races[0].RaceNumber != 3 || u.Races[0].Status != StatusRacing {
		t.Errorf("race 3 = %+v, want status Racing", u.Races[0])
	}
}

func TestAddFinishUsesFinishMilestoneAndMillis(t *testing.T) {
	var u UploadRequest
	u.AddFinish(7, 4, 6*time.Minute+12*time.Second+500*time.Millisecond)

	if len(u.Results) != 1 {
		t.Fatalf("results = %d, want 1", len(u.Results))
	}
	r := u.Results[0]
	if r.TimingMilestoneID != MilestoneFinish {
		t.Errorf("milestone = %d, want %d", r.TimingMilestoneID, MilestoneFinish)
	}
	if r.Time != 372500 {
		t.Errorf("time = %d ms, want 372500", r.Time)
	}
}
