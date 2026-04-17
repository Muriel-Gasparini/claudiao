package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Muriel-Gasparini/claudiao/internal/installer"
)

func keyMsg(k string) tea.KeyMsg {
	switch k {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
}

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	msg := cmd()
	_, ok := msg.(tea.QuitMsg)
	return ok
}

func TestDefaultModules(t *testing.T) {
	mods := defaultModules()
	if len(mods) != 5 {
		t.Fatalf("expected 5 modules, got %d", len(mods))
	}
	for _, m := range mods {
		if !m.Enabled {
			t.Errorf("module %s should default to enabled", m.ID)
		}
		if m.ID == "" || m.Name == "" || m.Desc == "" {
			t.Errorf("module missing fields: %+v", m)
		}
	}
}

func TestAnyEnabled(t *testing.T) {
	if anyEnabled([]Module{{Enabled: false}, {Enabled: false}}) {
		t.Error("expected false when none enabled")
	}
	if !anyEnabled([]Module{{Enabled: false}, {Enabled: true}}) {
		t.Error("expected true when one enabled")
	}
	if anyEnabled(nil) {
		t.Error("expected false for nil slice")
	}
}

func TestCountEnabled(t *testing.T) {
	mods := []Module{{Enabled: true}, {Enabled: false}, {Enabled: true}}
	if n := countEnabled(mods); n != 2 {
		t.Errorf("expected 2, got %d", n)
	}
}

func TestSelectedModuleIDs(t *testing.T) {
	m := Model{modules: []Module{
		{ID: "rules", Enabled: true},
		{ID: "agents", Enabled: false},
		{ID: "commands", Enabled: true},
	}}
	got := m.selectedModuleIDs()
	if len(got) != 2 || got[0] != "rules" || got[1] != "commands" {
		t.Errorf("unexpected ids: %v", got)
	}
}

func TestInstallerModeTranslation(t *testing.T) {
	if (Model{mode: ModeCopy}).installerMode() != installer.ModeCopy {
		t.Error("copy mode mismatch")
	}
	if (Model{mode: ModeSymlink}).installerMode() != installer.ModeSymlink {
		t.Error("symlink mode mismatch")
	}
}

func TestAssetsDirFromEnv(t *testing.T) {
	t.Setenv("CLAUDIAO_ASSETS_DIR", "/tmp/xyz")
	if got := assetsDir(); got != "/tmp/xyz" {
		t.Errorf("expected env var, got %s", got)
	}
}

func TestWelcomeContinueAdvances(t *testing.T) {
	m := Model{screen: screenWelcome, welcomeCursor: 0, modules: defaultModules()}
	updated, _ := m.Update(keyMsg("enter"))
	if updated.(Model).screen != screenModules {
		t.Errorf("expected screenModules, got %v", updated.(Model).screen)
	}
}

func TestWelcomeAbortQuits(t *testing.T) {
	m := Model{screen: screenWelcome, welcomeCursor: 1}
	_, cmd := m.Update(keyMsg("enter"))
	if !isQuit(cmd) {
		t.Error("expected quit cmd")
	}
}

func TestWelcomeQKeyQuits(t *testing.T) {
	m := Model{screen: screenWelcome}
	_, cmd := m.Update(keyMsg("q"))
	if !isQuit(cmd) {
		t.Error("expected quit cmd")
	}
}

func TestWelcomeCursorClamps(t *testing.T) {
	m := Model{screen: screenWelcome, welcomeCursor: 0}
	updated, _ := m.Update(keyMsg("up"))
	if updated.(Model).welcomeCursor != 0 {
		t.Error("cursor should not go below 0")
	}
	m2 := Model{screen: screenWelcome, welcomeCursor: 1}
	updated2, _ := m2.Update(keyMsg("down"))
	if updated2.(Model).welcomeCursor != 1 {
		t.Error("cursor should not exceed options")
	}
}

func TestModulesToggleSpace(t *testing.T) {
	m := Model{screen: screenModules, modules: []Module{
		{ID: "rules", Enabled: true},
		{ID: "commands", Enabled: true},
	}}
	updated, _ := m.Update(keyMsg("space"))
	got := updated.(Model)
	if got.modules[0].Enabled {
		t.Error("first module should be toggled off")
	}
	if !got.modules[1].Enabled {
		t.Error("second should be unchanged")
	}
}

func TestModulesToggleAll(t *testing.T) {
	m := Model{screen: screenModules, modules: []Module{
		{ID: "rules", Enabled: true},
		{ID: "commands", Enabled: true},
	}}
	updated, _ := m.Update(keyMsg("a"))
	got := updated.(Model)
	for _, mod := range got.modules {
		if mod.Enabled {
			t.Errorf("%s should be disabled after 'a' when all enabled", mod.ID)
		}
	}
	updated2, _ := updated.(Model).Update(keyMsg("a"))
	for _, mod := range updated2.(Model).modules {
		if !mod.Enabled {
			t.Errorf("%s should be enabled after second 'a'", mod.ID)
		}
	}
}

func TestModulesEnterAdvancesWhenAny(t *testing.T) {
	m := Model{screen: screenModules, modules: []Module{{ID: "rules", Enabled: true}}}
	updated, _ := m.Update(keyMsg("enter"))
	if updated.(Model).screen != screenMode {
		t.Error("expected screenMode after enter with enabled module")
	}
}

func TestModulesEnterStaysWhenNone(t *testing.T) {
	m := Model{screen: screenModules, modules: []Module{{ID: "rules", Enabled: false}}}
	updated, _ := m.Update(keyMsg("enter"))
	if updated.(Model).screen != screenModules {
		t.Error("should not advance when nothing enabled")
	}
}

func TestModulesBackToWelcome(t *testing.T) {
	m := Model{screen: screenModules}
	updated, _ := m.Update(keyMsg("b"))
	if updated.(Model).screen != screenWelcome {
		t.Error("expected screenWelcome after b")
	}
}

func TestModeEnterTriggersPlan(t *testing.T) {
	m := Model{screen: screenMode, modeCursor: 0, modules: []Module{{ID: "rules", Enabled: true}}, claudePath: "/tmp/fake"}
	updated, cmd := m.Update(keyMsg("enter"))
	if updated.(Model).screen != screenPreview {
		t.Error("expected screenPreview")
	}
	if cmd == nil {
		t.Error("expected a plan cmd")
	}
}

func TestModeBackToModules(t *testing.T) {
	m := Model{screen: screenMode}
	updated, _ := m.Update(keyMsg("b"))
	if updated.(Model).screen != screenModules {
		t.Error("expected screenModules after b")
	}
}

func TestModeSymlinkSelection(t *testing.T) {
	m := Model{screen: screenMode, modeCursor: 0, modules: []Module{{ID: "rules", Enabled: true}}, claudePath: "/tmp/fake"}
	updated, _ := m.Update(keyMsg("down"))
	updated2, _ := updated.(Model).Update(keyMsg("enter"))
	if updated2.(Model).mode != ModeSymlink {
		t.Error("expected symlink mode")
	}
}

func TestPreviewStoresPlanReadyMsg(t *testing.T) {
	m := Model{screen: screenPreview}
	plan := &installer.Plan{Actions: []installer.Action{{Source: "a", Target: "b", Kind: installer.KindCreate}}}
	updated, _ := m.Update(planReadyMsg{plan: plan})
	if updated.(Model).plan == nil {
		t.Error("plan should be stored")
	}
}

func TestPreviewStoresPlanError(t *testing.T) {
	m := Model{screen: screenPreview}
	updated, _ := m.Update(planReadyMsg{err: errExpected()})
	if updated.(Model).planErr == nil {
		t.Error("error should be stored")
	}
}

func TestPreviewBackClearsPlan(t *testing.T) {
	m := Model{screen: screenPreview, plan: &installer.Plan{}}
	updated, _ := m.Update(keyMsg("b"))
	got := updated.(Model)
	if got.screen != screenMode {
		t.Error("expected screenMode")
	}
	if got.plan != nil {
		t.Error("plan should be cleared on back")
	}
}

func TestPreviewInstallRequiresActions(t *testing.T) {
	m := Model{screen: screenPreview, plan: &installer.Plan{}}
	updated, cmd := m.Update(keyMsg("i"))
	if updated.(Model).screen != screenPreview {
		t.Error("should stay on preview with empty plan")
	}
	if cmd != nil {
		t.Error("should not start install with empty plan")
	}
}

func TestInstallingSpinnerAdvances(t *testing.T) {
	m := Model{screen: screenInstalling, spinnerFrame: 3}
	updated, cmd := m.Update(spinnerTickMsg{})
	if updated.(Model).spinnerFrame != 4 {
		t.Error("spinner should advance")
	}
	if cmd == nil {
		t.Error("should re-schedule spinner tick")
	}
}

func TestInstallingDoneTransitions(t *testing.T) {
	m := Model{screen: screenInstalling}
	updated, _ := m.Update(installDoneMsg{err: nil, backupPath: "/tmp/backup", written: 5})
	got := updated.(Model)
	if got.screen != screenResult {
		t.Error("expected screenResult")
	}
	if got.result.written != 5 {
		t.Errorf("expected 5 written, got %d", got.result.written)
	}
}

func TestResultEnterQuits(t *testing.T) {
	m := Model{screen: screenResult}
	_, cmd := m.Update(keyMsg("enter"))
	if !isQuit(cmd) {
		t.Error("expected quit cmd")
	}
}

func TestWindowSizeIsStored(t *testing.T) {
	m := Model{}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	got := updated.(Model)
	if got.width != 100 || got.height != 40 {
		t.Errorf("size mismatch: %d x %d", got.width, got.height)
	}
}

func TestViewsRenderWithoutPanic(t *testing.T) {
	m := New()
	m.width = 80
	m.height = 24
	screens := []screen{screenWelcome, screenModules, screenMode, screenPreview, screenInstalling, screenResult}
	for _, s := range screens {
		m.screen = s
		if out := m.View(); out == "" && s != screen(99) {
			t.Errorf("screen %d rendered empty", s)
		}
	}
}

func TestViewUnknownScreenEmpty(t *testing.T) {
	m := Model{screen: screen(99)}
	if got := m.View(); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestPreviewViewWithPlanRendersActions(t *testing.T) {
	m := Model{screen: screenPreview, claudePath: "/tmp/x", plan: &installer.Plan{
		Actions: []installer.Action{
			{Source: "rules/a.md", Target: "/tmp/x/rules/a.md", Kind: installer.KindCreate},
			{Source: "rules/b.md", Target: "/tmp/x/rules/b.md", Kind: installer.KindSame},
			{Source: "rules/c.md", Target: "/tmp/x/rules/c.md", Kind: installer.KindDiffer},
		},
	}}
	out := m.View()
	for _, want := range []string{"rules/a.md", "rules/b.md", "rules/c.md", "create", "same", "differ"} {
		if !contains(out, want) {
			t.Errorf("expected %q in output", want)
		}
	}
}

func TestPreviewViewWithErrorRendersError(t *testing.T) {
	m := Model{screen: screenPreview, planErr: errExpected()}
	out := m.View()
	if !contains(out, "expected failure") {
		t.Error("expected error message in view")
	}
}

func TestPreviewViewEmptyActions(t *testing.T) {
	m := Model{screen: screenPreview, plan: &installer.Plan{}}
	out := m.View()
	if !contains(out, "no files to install") {
		t.Error("expected empty state message")
	}
}

func TestPreviewViewScrollWindow(t *testing.T) {
	actions := make([]installer.Action, 30)
	for i := range actions {
		actions[i] = installer.Action{Source: "f", Target: "t", Kind: installer.KindCreate}
	}
	m := Model{screen: screenPreview, plan: &installer.Plan{Actions: actions}, previewCur: 20}
	out := m.View()
	if out == "" {
		t.Error("expected non-empty render with large plan")
	}
}

func TestPreviewInstallStartsWithValidPlan(t *testing.T) {
	m := Model{
		screen:     screenPreview,
		claudePath: "/tmp/fake",
		plan: &installer.Plan{
			Actions: []installer.Action{{Source: "x", Target: "/tmp/x", Kind: installer.KindCreate}},
		},
		modules: []Module{{ID: "rules", Enabled: true}},
	}
	updated, cmd := m.Update(keyMsg("i"))
	if updated.(Model).screen != screenInstalling {
		t.Error("expected screenInstalling")
	}
	if cmd == nil {
		t.Error("expected batched cmd")
	}
}

func TestPreviewCursorBounds(t *testing.T) {
	m := Model{screen: screenPreview, plan: &installer.Plan{
		Actions: []installer.Action{{Kind: installer.KindCreate}, {Kind: installer.KindCreate}},
	}}
	updated, _ := m.Update(keyMsg("up"))
	if updated.(Model).previewCur != 0 {
		t.Error("cursor should clamp at 0")
	}
	updated2, _ := m.Update(keyMsg("down"))
	if updated2.(Model).previewCur != 1 {
		t.Error("cursor should advance")
	}
	updated3, _ := updated2.(Model).Update(keyMsg("down"))
	if updated3.(Model).previewCur != 1 {
		t.Error("cursor should clamp at last")
	}
}

func TestPreviewQKeyQuits(t *testing.T) {
	m := Model{screen: screenPreview}
	_, cmd := m.Update(keyMsg("q"))
	if !isQuit(cmd) {
		t.Error("expected quit cmd")
	}
}

func TestResultViewErrorBranch(t *testing.T) {
	m := Model{screen: screenResult, result: installResult{err: errExpected(), backupPath: "/tmp/b"}}
	out := m.View()
	if !contains(out, "failed") || !contains(out, "expected failure") || !contains(out, "/tmp/b") {
		t.Errorf("expected error branch with backup, got: %s", out)
	}
}

func TestResultViewSuccessWithoutBackup(t *testing.T) {
	m := Model{screen: screenResult, claudePath: "/tmp/x", result: installResult{written: 3}}
	out := m.View()
	if !contains(out, "Done") || !contains(out, "3 files") {
		t.Errorf("expected success text, got: %s", out)
	}
}

func TestBuildPlanCmdReturnsMsg(t *testing.T) {
	t.Setenv("CLAUDIAO_ASSETS_DIR", "")
	m := Model{claudePath: "/tmp", modules: []Module{{ID: "rules", Enabled: true}}, mode: ModeCopy}
	cmd := buildPlanCmd(m)
	if cmd == nil {
		t.Fatal("expected cmd")
	}
	msg := cmd()
	if _, ok := msg.(planReadyMsg); !ok {
		t.Errorf("expected planReadyMsg, got %T", msg)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

type expErr struct{ msg string }

func (e expErr) Error() string { return e.msg }
func errExpected() error       { return expErr{"expected failure"} }
