package gateway

import (
	"encoding/json"
	"strings"
	"testing"
)

// Reworded asks count as the same ask; short inputs only when identical.
// The mutation check: compare exactly and the reworded ones stop counting.
func TestRewordedAsksCountAsTheSameAsk(t *testing.T) {
	earlier := []string{
		`{"append":"RULE (2026-09-19): \"better quality\" = sharper picture → re-encode at high bitrate (CRF 18, ~8 Mbps) with clip_convert, verify with ffprobe"}`,
		`{"append":"RULE (2026-09-19): user's \"better quality\" = sharper picture → re-encode high bitrate (CRF 18, ~8 Mbps) via clip_convert; verify with ffprobe"}`,
		`{"append":"PICK: Fireship lines 1-6, the researcher who quit before vesting; CNN dropped, one vertical short"}`,
	}
	input := `{"append":"RULE (2026-09-19): \"better quality\" on a reel = sharper picture → re-encode at high bitrate (CRF 18, ~8 Mbps) with clip_convert and verify with ffprobe"}`
	if n := nearSameAsks(earlier, input); n != 2 {
		t.Fatalf("two reworded repeats, got %d", n)
	}
	if n := nearSameAsks([]string{`{"read":true}`, `{"read":true}`}, `{"read":true}`); n != 2 {
		t.Fatalf("identical short inputs count, got %d", n)
	}
	if n := nearSameAsks([]string{`{"read":true}`}, `{"dir":"x"}`); n != 0 {
		t.Fatalf("different short inputs do not, got %d", n)
	}
	// Four fetches of four URLs are four asks: the shape is shared, the
	// one word that differs is the point. Same for candidates on sources.
	fetches := []string{
		`{"url":"https://www.youtube.com/watch?v=b-qNdFax69k","height":2160}`,
		`{"url":"https://www.youtube.com/watch?v=uWZqn5IA0Cw","height":2160}`,
		`{"url":"https://www.youtube.com/watch?v=Xm5NvPyUHFc","height":2160}`,
	}
	if n := nearSameAsks(fetches, `{"url":"https://www.youtube.com/watch?v=nlaNalq7D0k","height":2160}`); n != 0 {
		t.Fatalf("a fetch of another URL is another ask, got %d", n)
	}
	if n := nearSameAsks(fetches, `{"url":"https://www.youtube.com/watch?v=Xm5NvPyUHFc","height":2160}`); n != 1 {
		t.Fatalf("the same URL again is the same ask, got %d", n)
	}
	cands := []string{`{"source":"a.mp4","topic":"promesse candidat retraite 2027","count":8}`, `{"source":"b.mp4","topic":"promesse candidat retraite 2027","count":8}`}
	if n := nearSameAsks(cands, `{"source":"c.mp4","topic":"promesse candidat retraite 2027","count":8}`); n != 0 {
		t.Fatalf("candidates on another source is another ask, got %d", n)
	}
}

// The end of a stuck turn names the last failure: tool and reason, one
// line, or nothing when nothing failed.
func TestAStuckTurnEndsNamingTheLastFailure(t *testing.T) {
	ran := []ToolExecutionInfo{
		{Name: "clip_fetch", Output: "ok"},
		{Name: "bash", Error: "transcripts are not for reading whole (they fill the window and end the film): look at up to 40 lines at a time\nsecond line"},
		{Name: "notes", Output: "noted"},
	}
	got := lastToolFailure(ran)
	if !strings.HasPrefix(got, " — bash: transcripts are not for reading whole") || strings.Contains(got, "second line") {
		t.Fatalf("want the last failure, one line: %q", got)
	}
	if lastToolFailure([]ToolExecutionInfo{{Name: "x", Output: "y"}}) != "" {
		t.Fatal("nothing failed, nothing named")
	}
}

// Reads of different files in one deep project are different asks, however
// many words their absolute paths share; the same file twice is the same ask.
func TestDifferentFilesOfOneProjectAreNotTheSameAsk(t *testing.T) {
	dir := `I:\Projects\temp\benchmark.ai\qwen\backend\DispatchLab.Core\Domain\`
	read := func(name string) string {
		b, _ := json.Marshal(map[string]any{"all": true, "path": dir + name})
		return string(b)
	}
	var earlier []string
	for _, f := range []string{"CommandEngine.cs", "Orders.cs", "Common.cs", "CommandValidation.cs", "ReadServices.cs", "SimulationClock.cs"} {
		earlier = append(earlier, read(f))
	}
	if n := nearSameAsks(earlier, read("Allocator.cs")); n != 0 {
		t.Fatalf("a seventh different file counted as %d repeats", n)
	}
	if n := nearSameAsks(earlier, read("Orders.cs")); n != 1 {
		t.Fatalf("the same file again: %d", n)
	}
}

// The loop-breaker says a failure only when one tripped it.
func TestTheLoopBreakerNamesAFailureOnlyWhenThereWasOne(t *testing.T) {
	ok := []ToolExecutionInfo{{Name: "read_file", Output: "x"}}
	if s := loopStopLine(ok, false); strings.Contains(s, "failing") {
		t.Fatalf("repeat called a failure: %q", s)
	}
	if s := loopNudge(ok, false); strings.Contains(s, "failing") {
		t.Fatalf("repeat nudge called a failure: %q", s)
	}
	bad := []ToolExecutionInfo{{Name: "apply_patch", Error: "anchor not found"}}
	if s := loopStopLine(bad, true); !strings.Contains(s, "kept failing") || !strings.Contains(s, "anchor not found") {
		t.Fatalf("failure line: %q", s)
	}
}
