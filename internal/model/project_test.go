package model

import "testing"

// A project saved under the inch-named iPhone slots must come out of Migrate
// on the slots that replaced them, with a version that held both old ids and
// the new one collapsing to one target per slot rather than exporting twice.
func TestMigrateRenamesRetiredSizes(t *testing.T) {
	p := Project{
		Settings: Settings{SizeID: "iphone-6-9"},
		Versions: []Version{{ID: "v_1", Name: "1.0", Targets: []Target{
			{SizeID: "iphone-6-9", DeviceID: "iphone-17-pro"},
			{SizeID: "iphone-6-5", DeviceID: "iphone-17"},
			{SizeID: "iphone-island-medium", DeviceID: "iphone-17"},
			{SizeID: "ipad-13", DeviceID: "ipad-13"},
		}}},
		VersionID: "v_1",
	}
	p.Migrate()

	if p.Settings.SizeID != "iphone-island-medium" {
		t.Errorf("settings size: got %q", p.Settings.SizeID)
	}
	var got []string
	for _, tg := range p.Targets() {
		got = append(got, tg.SizeID)
	}
	want := []string{"iphone-island-medium", "iphone-faceid-medium", "ipad-13"}
	if len(got) != len(want) {
		t.Fatalf("targets: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("targets: got %v, want %v", got, want)
		}
	}
	// The frame chosen for the slot is the author's, and survives the rename.
	if d := p.Targets()[0].DeviceID; d != "iphone-17-pro" {
		t.Errorf("first target kept frame %q, want iphone-17-pro", d)
	}
}
