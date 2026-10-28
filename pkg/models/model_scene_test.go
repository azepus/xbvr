package models

import (
	"testing"

	"github.com/jinzhu/gorm"
)

func TestGetClimaxMinute(t *testing.T) {
	tests := []struct {
		name     string
		scene    *Scene
		expected int
	}{
		{
			name:     "nil scene",
			scene:    nil,
			expected: 0,
		},
		{
			name:     "empty tags",
			scene:    &Scene{Tags: []Tag{}},
			expected: 0,
		},
		{
			name: "standard climax tag with m suffix",
			scene: &Scene{
				Tags: []Tag{
					{Name: "POV"},
					{Name: "climax-18m"},
					{Name: "Blonde"},
				},
			},
			expected: 18,
		},
		{
			name: "climax tag single digit",
			scene: &Scene{
				Tags: []Tag{
					{Name: "climax-8m"},
				},
			},
			expected: 8,
		},
		{
			name: "climax tag without m suffix",
			scene: &Scene{
				Tags: []Tag{
					{Name: "climax-18"},
				},
			},
			expected: 18,
		},
		{
			name: "climax tag uppercase",
			scene: &Scene{
				Tags: []Tag{
					{Name: "CLIMAX-25M"},
				},
			},
			expected: 25,
		},
		{
			name: "climax tag with whitespace",
			scene: &Scene{
				Tags: []Tag{
					{Name: "  climax-14m  "},
				},
			},
			expected: 14,
		},
		{
			name: "climax tag zero minutes",
			scene: &Scene{
				Tags: []Tag{
					{Name: "climax-0m"},
				},
			},
			expected: 0,
		},
		{
			name: "non-climax tags",
			scene: &Scene{
				Tags: []Tag{
					{Name: "climax"},
					{Name: "climax-"},
					{Name: "climax-abc"},
					{Name: "anticlimax-18m"},
				},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.scene.GetClimaxMinute()
			if result != tt.expected {
				t.Errorf("GetClimaxMinute() = %d, expected %d", result, tt.expected)
			}
		})
	}
}

func TestUpdateClimaxScore(t *testing.T) {
	testDb, err := gorm.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite db: %v", err)
	}
	defer testDb.Close()

	if err := testDb.AutoMigrate(&Scene{}, &Tag{}, &File{}).Error; err != nil {
		t.Fatalf("failed to automigrate models: %v", err)
	}

	oldConn := commonConnection
	commonConnection = testDb
	defer func() { commonConnection = oldConn }()

	// Test nil scene
	var nilScene *Scene
	beaten, oldRec, err := nilScene.UpdateClimaxScore(10)
	if err != nil || beaten || oldRec != 0 {
		t.Errorf("expected (false, 0, nil) for nil scene, got (%v, %d, %v)", beaten, oldRec, err)
	}

	// Create test scene
	scene := Scene{
		Title: "Test Scene",
	}
	if err := testDb.Create(&scene).Error; err != nil {
		t.Fatalf("failed to create scene: %v", err)
	}

	// Create associated script file
	scriptFile := File{
		SceneID:             scene.ID,
		Filename:            "test_script.funscript",
		Type:                "script",
		RefreshHeatmapCache: false,
	}
	if err := testDb.Create(&scriptFile).Error; err != nil {
		t.Fatalf("failed to create script file: %v", err)
	}

	// Test minutes <= 0 (ignored)
	beaten, oldRec, err = scene.UpdateClimaxScore(0)
	if err != nil || beaten || oldRec != 0 {
		t.Errorf("expected (false, 0, nil) for 0 minutes, got (%v, %d, %v)", beaten, oldRec, err)
	}
	beaten, oldRec, err = scene.UpdateClimaxScore(-5)
	if err != nil || beaten || oldRec != 0 {
		t.Errorf("expected (false, 0, nil) for negative minutes, got (%v, %d, %v)", beaten, oldRec, err)
	}

	// Cas 1: Aucun record existant (oldRecord == 0) -> 15 min
	beaten, oldRec, err = scene.UpdateClimaxScore(15)
	if err != nil {
		t.Fatalf("unexpected error on initial climax score: %v", err)
	}
	if !beaten {
		t.Errorf("expected beaten=true for initial climax, got false")
	}
	if oldRec != 0 {
		t.Errorf("expected oldRecord=0, got %d", oldRec)
	}
	if cur := scene.GetClimaxMinute(); cur != 15 {
		t.Errorf("expected scene climax to be 15, got %d", cur)
	}

	// Verify script file RefreshHeatmapCache was set to true
	var reloadedFile File
	if err := testDb.First(&reloadedFile, scriptFile.ID).Error; err != nil {
		t.Fatalf("failed to reload script file: %v", err)
	}
	if !reloadedFile.RefreshHeatmapCache {
		t.Errorf("expected RefreshHeatmapCache=true on script file")
	}

	// Reset RefreshHeatmapCache on file to test next update
	reloadedFile.RefreshHeatmapCache = false
	testDb.Save(&reloadedFile)

	// Cas 3: Performance inférieure ou égale (10 <= 15) -> Ne rien modifier
	beaten, oldRec, err = scene.UpdateClimaxScore(10)
	if err != nil {
		t.Fatalf("unexpected error on lower climax score: %v", err)
	}
	if beaten {
		t.Errorf("expected beaten=false for lower score, got true")
	}
	if oldRec != 15 {
		t.Errorf("expected oldRecord=15, got %d", oldRec)
	}
	if cur := scene.GetClimaxMinute(); cur != 15 {
		t.Errorf("expected scene climax to remain 15, got %d", cur)
	}

	// Cas 3: Performance égale (15 <= 15) -> Ne rien modifier
	beaten, oldRec, err = scene.UpdateClimaxScore(15)
	if err != nil {
		t.Fatalf("unexpected error on equal climax score: %v", err)
	}
	if beaten {
		t.Errorf("expected beaten=false for equal score, got true")
	}
	if oldRec != 15 {
		t.Errorf("expected oldRecord=15, got %d", oldRec)
	}

	// Cas 2: Nouveau record battu (20 > 15)
	beaten, oldRec, err = scene.UpdateClimaxScore(20)
	if err != nil {
		t.Fatalf("unexpected error on new high score: %v", err)
	}
	if !beaten {
		t.Errorf("expected beaten=true for new high score, got false")
	}
	if oldRec != 15 {
		t.Errorf("expected oldRecord=15, got %d", oldRec)
	}
	if cur := scene.GetClimaxMinute(); cur != 20 {
		t.Errorf("expected scene climax to be 20, got %d", cur)
	}

	// Verify old tag climax-15m was removed and climax-20m exists in DB
	var dbScene Scene
	testDb.Preload("Tags").First(&dbScene, scene.ID)
	hasOldTag := false
	hasNewTag := false
	for _, tTag := range dbScene.Tags {
		if tTag.Name == "climax-15m" {
			hasOldTag = true
		}
		if tTag.Name == "climax-20m" {
			hasNewTag = true
		}
	}
	if hasOldTag {
		t.Errorf("old tag climax-15m should have been removed from scene tags")
	}
	if !hasNewTag {
		t.Errorf("new tag climax-20m should be associated with scene")
	}

	// Verify RefreshHeatmapCache is true again
	testDb.First(&reloadedFile, scriptFile.ID)
	if !reloadedFile.RefreshHeatmapCache {
		t.Errorf("expected RefreshHeatmapCache=true on script file after beating record")
	}
}
