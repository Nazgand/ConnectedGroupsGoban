package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Width returns the board width of the current game, or 0 if no game is selected.
func (GoWin *GoWin) Width() uint8 {
	if GoWin.Coll.CurrentGameH == nil {
		return 0
	}
	return GoWin.Coll.CurrentGameH.Width
}

// Height returns the board height of the current game, or 0 if no game is selected.
func (GoWin *GoWin) Height() uint8 {
	if GoWin.Coll.CurrentGameH == nil {
		return 0
	}
	return GoWin.Coll.CurrentGameH.Height
}

// LoadConfig reads the JSON config file from ConfigDir, decodes it into
// CurrentAppConfig (starting from DefaultConfig for scalar defaults), removes
// the "Default" theme (always regenerated), and validates/fills missing fields.
func LoadConfig() error {
	ConfigPath := filepath.Join(filepath.Dir(ConfigDir), ConfigFile)

	File, Err := os.Open(ConfigPath)
	if Err != nil {
		return Err
	}
	defer File.Close()

	fmt.Println("Opened config")

	// Start from DefaultConfig for scalar defaults, but nil the maps so the
	// JSON decoder allocates fresh maps instead of merging into DefaultConfig's
	// shared map instances (which would mutate DefaultConfig permanently).
	Config := DefaultConfig
	Config.Themes = nil
	Config.Engines = nil
	Config.FreshBoardPresets = nil
	Config.ShownProtectedRuleSetNames = nil
	Config.ShownCustomRuleSets = nil
	Config.HiddenCustomRuleSets = nil
	Decoder := json.NewDecoder(File)
	Err = Decoder.Decode(&Config)
	if Err != nil {
		return Err
	}

	// Apply the loaded configuration
	CurrentAppConfig = Config
	// "Default" theme is always regenerated from DefaultConfig on load,
	// ignoring whatever the config file says.
	if CurrentAppConfig.Themes == nil {
		CurrentAppConfig.Themes = make(map[string]Theme)
	}
	CurrentAppConfig.Themes["Default"] = DefaultConfig.Themes["Default"]

	ValidateAndFillConfig()

	return nil
}

// ValidateAndFillConfig ensures all config fields have valid values by filling missing/invalid ones with defaults
func ValidateAndFillConfig() {
	// Ensure Engines map exists and has default if empty
	if CurrentAppConfig.Engines == nil {
		CurrentAppConfig.Engines = make(map[string]EngineConfig)
	}
	if len(CurrentAppConfig.Engines) == 0 {
		CurrentAppConfig.Engines["Default"] = EngineConfig{
			GtpPath: "gnugo",
			GtpArgs: "--mode gtp --level 5 --chinese-rules",
		}
	}

	// Ensure FreshBoardPresets map exists and has default
	if CurrentAppConfig.FreshBoardPresets == nil {
		CurrentAppConfig.FreshBoardPresets = make(map[string]FreshBoardPreset)
	}
	// "Default" preset is always regenerated from DefaultConfig; never persisted.
	CurrentAppConfig.FreshBoardPresets["Default"] = DefaultConfig.FreshBoardPresets["Default"]

	// Upgrade preset RuleSets that match a protected rule set by name:
	// replace with the canonical version so stale NonRepetitionRules etc. are fixed.
	for PresetName, Preset := range CurrentAppConfig.FreshBoardPresets {
		if Preset.RuleSet != nil && len(Preset.RuleSet.Names) > 0 {
			if Protected := FindRuleSetByName(Preset.RuleSet.Names[0]); Protected != nil {
				Preset.RuleSet = Protected
				CurrentAppConfig.FreshBoardPresets[PresetName] = Preset
			}
		}
	}

	// Ensure ActivePresetName is valid
	if CurrentAppConfig.ActivePresetName == "" {
		CurrentAppConfig.ActivePresetName = "Default"
	}
	if _, exists := CurrentAppConfig.FreshBoardPresets[CurrentAppConfig.ActivePresetName]; !exists {
		CurrentAppConfig.ActivePresetName = "Default"
	}

	// Ensure Themes map exists and always has the canonical Default theme.
	if CurrentAppConfig.Themes == nil {
		CurrentAppConfig.Themes = make(map[string]Theme)
	}
	CurrentAppConfig.Themes["Default"] = DefaultConfig.Themes["Default"]

	// Ensure ActiveTheme is valid
	if CurrentAppConfig.ActiveTheme == "" {
		CurrentAppConfig.ActiveTheme = "Default"
	}
	if _, exists := CurrentAppConfig.Themes[CurrentAppConfig.ActiveTheme]; !exists {
		CurrentAppConfig.ActiveTheme = "Default"
	}

	// Validate ShownProtectedRuleSetNames: keep only names that resolve
	// to a known protected rule set. A nil slice means the field was absent
	// and should receive defaults; an explicitly empty slice means the user
	// hid every built-in rule set from the score table.
	ProtectedRuleSetNamesWereMissing := CurrentAppConfig.ShownProtectedRuleSetNames == nil
	ValidNames := []string{}
	for _, Name := range CurrentAppConfig.ShownProtectedRuleSetNames {
		if FindRuleSetByName(Name) != nil {
			ValidNames = append(ValidNames, Name)
		}
	}
	if ProtectedRuleSetNamesWereMissing {
		ValidNames = DefaultConfig.ShownProtectedRuleSetNames
	}
	CurrentAppConfig.ShownProtectedRuleSetNames = ValidNames

	// Ensure custom rule set slices are non-nil.
	if CurrentAppConfig.ShownCustomRuleSets == nil {
		CurrentAppConfig.ShownCustomRuleSets = []RuleSet{}
	}
	if CurrentAppConfig.HiddenCustomRuleSets == nil {
		CurrentAppConfig.HiddenCustomRuleSets = []RuleSet{}
	}

	// Enforce the invariant: no two custom rule sets are content-equal. Merges
	// any content-duplicates (preferring Shown) and re-links preset pointers.
	DeduplicateCustomRuleSets()

	// Ensure PlayerNames exists
	if CurrentAppConfig.PlayerNames == nil {
		CurrentAppConfig.PlayerNames = []string{}
	}

}

// SnapshotCurrentConfigBytes records CurrentAppConfig's current serialized
// form as the "last saved" baseline. Call after LoadConfig (success or fail)
// so subsequent SaveConfigSnapshot calls skip writes until the config actually
// changes. When no config file exists and the user makes no changes, this
// prevents writing a config identical to the default on startup.
func SnapshotCurrentConfigBytes() {
	var Buf bytes.Buffer
	Encoder := json.NewEncoder(&Buf)
	Encoder.SetIndent("", "  ")
	if Err := Encoder.Encode(CurrentAppConfig); Err != nil {
		LastSavedConfigBytes = nil
		return
	}
	LastSavedConfigBytes = Buf.Bytes()
}

// SaveConfigAsync snapshots CurrentAppConfig and writes it to disk in a
// goroutine so the caller (e.g. a resize event on the UI thread) is never
// blocked by file I/O. Errors are printed to stdout.
func (GoWin *GoWin) SaveConfigAsync() {
	Snapshot := CurrentAppConfig
	go func() {
		if Err := SaveConfigSnapshot(Snapshot); Err != nil {
			fmt.Println("Failed to save config:", Err)
		}
	}()
}

// SaveConfigSnapshot writes a given AppConfig snapshot to the JSON config file.
// Skips the write entirely if the serialized form matches LastSavedConfigBytes.
func SaveConfigSnapshot(Config AppConfig) error {
	var Buf bytes.Buffer
	Encoder := json.NewEncoder(&Buf)
	Encoder.SetIndent("", "  ")
	if Err := Encoder.Encode(Config); Err != nil {
		return Err
	}
	NewBytes := Buf.Bytes()
	if bytes.Equal(NewBytes, LastSavedConfigBytes) {
		return nil
	}

	ConfigPath := filepath.Join(filepath.Dir(ConfigDir), ConfigFile)
	if Err := os.WriteFile(ConfigPath, NewBytes, 0644); Err != nil {
		return Err
	}
	LastSavedConfigBytes = NewBytes
	fmt.Println("Saved config")
	return nil
}

// SaveConfig writes the current CurrentAppConfig to the JSON config file
// in ConfigDir with pretty-printed indentation.
func (GoWin *GoWin) SaveConfig() error {
	return SaveConfigSnapshot(CurrentAppConfig)
}
