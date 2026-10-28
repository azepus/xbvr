package models

import (
	"testing"
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
