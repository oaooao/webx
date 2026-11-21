package egolite

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

// listTaskSpaces() as it stood on 2026-11-21 (trimmed): the "webx" space,
// id 3, had been handed to the user after a local-network permission prompt
// and stayed agentDelegatedToUser. Every webx run reused it and failed with
// "A browser permission prompt ... The user now controls this task space".
const spaces20261121 = `[
 {"createdBy":"user","id":0,"name":"","ownership":"user"},
 {"createdBy":"agent","id":3,"name":"webx","ownership":"agentDelegatedToUser"},
 {"createdBy":"agent","id":22,"name":"youtube transcript research","ownership":"agentDelegatedToUser"}
]`

type pickedSpace struct {
	Name    string         `json:"name"`
	Skipped []SkippedSpace `json:"skipped"`
}

// runPickSpace evaluates pickSpaceJS in Node. The browser script runs in
// ego-browser's embedded Node runtime, so Node is the closest stand-in.
func runPickSpace(t *testing.T, spaces string) pickedSpace {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not on PATH; pickSpaceJS is only checked where Node is available")
	}
	prog := pickSpaceJS + "\nconsole.log(JSON.stringify(pickSpace(" + spaces + `, "` + SpaceName + `")));`
	out, err := exec.Command(node, "-e", prog).CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, out)
	}
	var p pickedSpace
	if err := json.Unmarshal(out, &p); err != nil {
		t.Fatalf("pickSpace output %q: %v", out, err)
	}
	return p
}

// Regression for 2026-11-21: space "webx" (id 3) held by the user must not be
// reused; the run moves to a new space and reports the skipped one.
func TestPickSpace_WhenWebxHeldByUser_UsesNewSpace(t *testing.T) {
	p := runPickSpace(t, spaces20261121)
	if p.Name != "webx-2" {
		t.Fatalf("picked %q, want webx-2", p.Name)
	}
	if len(p.Skipped) != 1 || p.Skipped[0].ID != 3 || p.Skipped[0].Ownership != "agentDelegatedToUser" {
		t.Errorf("skipped = %+v, want space 3 (agentDelegatedToUser)", p.Skipped)
	}
}

func TestPickSpace_ReusesAgentHeldSpaces(t *testing.T) {
	if p := runPickSpace(t, `[{"id":3,"name":"webx","ownership":"agent"}]`); p.Name != "webx" || len(p.Skipped) != 0 {
		t.Errorf("agent-held webx: got %+v, want webx with nothing skipped", p)
	}
	if p := runPickSpace(t, `[]`); p.Name != "webx" {
		t.Errorf("no spaces: got %q, want webx", p.Name)
	}
	// webx stuck with the user, webx-2 already ours from an earlier run.
	p := runPickSpace(t, `[{"id":3,"name":"webx","ownership":"agentDelegatedToUser"},{"id":9,"name":"webx-2","ownership":"agent"},{"id":5,"name":"webx-3","ownership":"user"}]`)
	if p.Name != "webx-2" {
		t.Errorf("got %q, want webx-2 (reuse our own fallback space)", p.Name)
	}
}

func TestScripts_PickSpaceBeforeTaskSpace(t *testing.T) {
	run, _ := BuildScript(testJob())
	page, _ := BuildPageScript(PageJob{Site: "x.com", URL: "https://x.com/"})
	for name, script := range map[string]string{"fetch": run, "page": page} {
		for _, want := range []string{"listTaskSpaces()", "pickSpace(", "taskSpace(pick.name)"} {
			if !strings.Contains(script, want) {
				t.Errorf("%s script missing %q", name, want)
			}
		}
	}
}

// When the script reports a skipped space, the run still succeeds and webx
// tells the caller, in one line, which space waits for the user.
func TestRun_SkippedSpace_IsReportedOnce(t *testing.T) {
	var notes []string
	old := Notify
	t.Cleanup(func() { Notify = old })
	Notify = func(msg string) { notes = append(notes, msg) }

	stderr := resultMarker + `{"ok":true,"space":{"name":"webx-2","skipped":[{"name":"webx","id":3,"ownership":"agentDelegatedToUser"}]},"result":{"responses":[{"status":200,"url":"u","body":"{}"}],"logged_in":false,"timing":{}}}`
	if _, err := RunWith(context.Background(), fixedRunner("", stderr, nil), testJob()); err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 || !strings.Contains(notes[0], `"webx" (id 3`) || !strings.Contains(notes[0], `"webx-2"`) {
		t.Errorf("notes = %q, want one line naming webx (id 3) and webx-2", notes)
	}
}

// The prompt can also appear mid-run (the space is handed over during goto).
// The error must say what happened and that a retry moves to a new space,
// not just echo the prompt text.
func TestRun_SpaceHandedToUserMidRun_ExplainsRetry(t *testing.T) {
	stderr := resultMarker + `{"ok":false,"space":{"name":"webx","skipped":[]},"error":{"kind":"script","message":"Error: A browser permission prompt for local network access has appeared. The user now controls this task space."}}`
	_, err := RunWith(context.Background(), fixedRunner("", stderr, nil), testJob())
	if err == nil || !strings.Contains(err.Error(), "handed to the user") || !strings.Contains(err.Error(), "new space") {
		t.Errorf("error = %v, want an explanation that the space was handed to the user and a retry uses a new space", err)
	}
}
