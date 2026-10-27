package tasks

import (
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lucasb-eyer/go-colorful"
	"github.com/sirupsen/logrus"
	"github.com/xbapps/xbvr/pkg/common"
	"github.com/xbapps/xbvr/pkg/models"
)

// Script is the Funscript container type holding Launch data.
type Script struct {
	// Version of Launchscript
	Version interface{} `json:"version"`
	// Inverted causes up and down movement to be flipped.
	Inverted bool `json:"inverted,omitempty"`
	// Range is the percentage of a full stroke to use.
	Range int `json:"range,omitempty"`
	// Actions are the timed moves.
	Actions []Action `json:"actions"`
	// Metadata of the Funscript.
	Metadata *ScriptMetadata `json:"metadata,omitempty"`
}

// Action is a move at a specific time.
type Action struct {
	// At time in milliseconds the action should fire.
	At int64 `json:"at"`
	// Pos is the place in percent to move to.
	Pos int `json:"pos"`

	Slope     float64
	Intensity int64
}

// Metadata of a Funscript
type ScriptMetadata struct {
	// Duration of the scripted video, in seconds.
	Duration int64 `json:"duration,omitempty"`
}

type GradientTable []struct {
	Col colorful.Color
	Pos float64
}

func GenerateHeatmaps(tlog *logrus.Entry) {
	if !models.CheckLock("heatmaps") {
		models.CreateLock("heatmaps")
		defer models.RemoveLock("heatmaps")

		db, _ := models.GetDB()
		defer db.Close()

		var scriptfiles []models.File
		db.Model(&models.File{}).Preload("Volume").Where("type = ?", "script").Where("has_heatmap = ? OR script_speed = ? OR script_speed IS NULL OR script_apm = ? OR script_apm IS NULL", false, 0, 0).Find(&scriptfiles)

		for i, file := range scriptfiles {
			if tlog != nil && (i%50) == 0 {
				tlog.Infof("Generating heatmaps (%v/%v)", i+1, len(scriptfiles))
			}
			if file.Exists() {
				path := file.GetPath()
				if strings.HasSuffix(path, ".funscript") {
					funscript, err := LoadFunscriptData(path)
					speed := 0
					if err != nil {
						log.Warnf("Could not load funscript data for %s: %v", file.Filename, err)
					} else {
						if funscript.IsFunscriptToken() {
							log.Warnf("Funscript is a token: %s - speed set to 0", file.Filename)
						} else {
							speed = funscript.CalculateSpeed()
						}
					}
					file.ScriptSpeed = speed
					file.ScriptAPM = speed

					if !file.HasHeatmap {
						log.Infof("Rendering %v", file.Filename)
						destFile := filepath.Join(common.ScriptHeatmapDir, fmt.Sprintf("heatmap-%d.png", file.ID))
						err := RenderHeatmap(
							path,
							destFile,
							1000,
							10,
							250,
						)
						if err == nil {
							file.HasHeatmap = true
							file.RefreshHeatmapCache = true
						} else {
							log.Warn(err)
						}
					}
					file.Save()

					// Propagate ScriptSpeed to scene if this file is the primary script
					if file.SceneID != 0 {
						var scene models.Scene
						if err := scene.GetIfExistByPK(file.SceneID); err == nil {
							scripts, err := scene.GetScriptFiles()
							if err == nil && len(scripts) > 0 {
								if scripts[0].ID == file.ID {
									if scene.ScriptSpeed != file.ScriptSpeed || scene.ScriptAPM != file.ScriptAPM {
										scene.ScriptSpeed = file.ScriptSpeed
										scene.ScriptAPM = file.ScriptAPM
										scene.Save()
									}
								}
							}
						}
					}
				}
			}
		}

		// Ensure all scripted scenes have their script_speed and script_apm synchronized
		var scriptedScenes []models.Scene
		db.Model(&models.Scene{}).Where("is_scripted = ?", true).Where("script_speed = ? OR script_speed IS NULL OR script_apm = ? OR script_apm IS NULL", 0, 0).Find(&scriptedScenes)
		for i := range scriptedScenes {
			scriptedScenes[i].UpdateStatus()
		}
	}
}

func LoadFunscriptData(path string) (Script, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Script{}, err
	}

	var funscript Script
	err = json.Unmarshal(data, &funscript)
	if err != nil {
		return Script{}, err
	}

	if funscript.Actions == nil {
		return Script{}, fmt.Errorf("actions list missing in %s", path)
	}

	if len(funscript.Actions) == 0 {
		return Script{}, fmt.Errorf("actions list empty in %s", path)
	}

	sort.SliceStable(funscript.Actions, func(i, j int) bool { return funscript.Actions[i].At < funscript.Actions[j].At })

	// fix strokes with negative timestamps
	i := 0
	for i < len(funscript.Actions) && funscript.Actions[i].At < 0 {
		funscript.Actions[i].At = 0
		i += 1
	}

	return funscript, nil
}

func RenderHeatmap(inputFile string, destFile string, width, height, numSegments int) error {
	funscript, err := LoadFunscriptData(inputFile)
	if err != nil {
		return err
	}
	if funscript.IsFunscriptToken() {
		return fmt.Errorf("funscript is a token: %s - heatmap can't be rendered", inputFile)
	}

	funscript.UpdateIntensity()
	gradient := funscript.getGradientTable(numSegments)

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		c := gradient.GetInterpolatedColorFor(float64(x) / float64(width))
		draw.Draw(img, image.Rect(x, 0, x+1, height), &image.Uniform{c}, image.Point{}, draw.Src)
	}

	// add 10 minute marks
	maxts := funscript.Actions[len(funscript.Actions)-1].At
	const tick = 600000
	var ts int64 = tick
	c, _ := colorful.Hex("#000000")
	for ts < maxts {
		x := int(float64(ts) / float64(maxts) * float64(width))
		draw.Draw(img, image.Rect(x-1, height/2, x+1, height), &image.Uniform{c}, image.Point{}, draw.Src)
		ts += tick
	}

	outpng, err := os.Create(destFile)
	if err != nil {
		return fmt.Errorf("Error storing png: %v", err)
	}
	defer outpng.Close()

	png.Encode(outpng, img)
	return nil
}

func (gt GradientTable) GetInterpolatedColorFor(t float64) colorful.Color {
	for i := 0; i < len(gt)-1; i++ {
		c1 := gt[i]
		c2 := gt[i+1]
		if c1.Pos <= t && t <= c2.Pos {
			// We are in between c1 and c2. Go blend them!
			t := (t - c1.Pos) / (c2.Pos - c1.Pos)
			return c1.Col.BlendHcl(c2.Col, t).Clamped()
		}
	}

	// Nothing found? Means we're at (or past) the last gradient keypoint.
	return gt[len(gt)-1].Col
}

func (funscript Script) UpdateIntensity() {

	var t1, t2 int64
	var p1, p2 int
	var slope float64
	for i := range funscript.Actions {
		if i == 0 {
			continue
		}
		t1 = funscript.Actions[i].At
		t2 = funscript.Actions[i-1].At
		p1 = funscript.Actions[i].Pos
		p2 = funscript.Actions[i-1].Pos

		slope = math.Min(math.Max(1/(2*float64(t1-t2)/1000), 0), 20)

		funscript.Actions[i].Slope = slope
		funscript.Actions[i].Intensity = int64(slope * math.Abs((float64)(p1-p2)))
	}
}

func getSegmentColor(intensity float64) colorful.Color {
	colorBlue, _ := colorful.Hex("#1e90ff")   // DodgerBlue
	colorGreen, _ := colorful.Hex("#228b22")  // ForestGreen
	colorYellow, _ := colorful.Hex("#ffd700") // Gold
	colorRed, _ := colorful.Hex("#dc143c")    // Crimson
	colorPurple, _ := colorful.Hex("#800080") // Purple
	colorBlack, _ := colorful.Hex("#0f001e")
	colorWhite, _ := colorful.Hex("#ffffff")

	var stepSize float64 = 60.0
	var f float64
	var c colorful.Color

	if intensity <= 0.001 {
		c = colorWhite
	} else if intensity <= 1*stepSize {
		f = (intensity - 0*stepSize) / stepSize
		c = colorBlue.BlendLab(colorGreen, f)
	} else if intensity <= 2*stepSize {
		f = (intensity - 1*stepSize) / stepSize
		c = colorGreen.BlendLab(colorYellow, f)
	} else if intensity <= 3*stepSize {
		f = (intensity - 2*stepSize) / stepSize
		c = colorYellow.BlendLab(colorRed, f)
	} else if intensity <= 4*stepSize {
		f = (intensity - 3*stepSize) / stepSize
		c = colorRed.BlendRgb(colorPurple, f)
	} else {
		f = (intensity - 4*stepSize) / (5 * stepSize)
		f = math.Min(f, 1.0)
		c = colorPurple.BlendLab(colorBlack, f)
	}
	return c
}

func (funscript Script) getGradientTable(numSegments int) GradientTable {
	segments := make([]struct {
		count     int
		intensity int
	}, numSegments)
	gradient := make(GradientTable, numSegments)

	maxts := funscript.getDuration() * 1000.0

	for _, a := range funscript.Actions {
		segment := int(float64(a.At) / float64(maxts+1) * float64(numSegments))
		segments[segment].count = segments[segment].count + 1
		segments[segment].intensity = segments[segment].intensity + int(a.Intensity)
	}

	for i := 0; i < numSegments; i++ {
		gradient[i].Pos = float64(i) / float64(numSegments-1)
		if segments[i].count > 0 {
			gradient[i].Col = getSegmentColor(float64(segments[i].intensity) / float64(segments[i].count))
		} else {
			gradient[i].Col = getSegmentColor(0.0)
		}
	}

	return gradient
}

func (funscript *Script) IsFunscriptToken() bool {
	if len(funscript.Actions) == 0 || len(funscript.Actions) > 100 {
		return false
	}
	actions := make([]Action, len(funscript.Actions))
	copy(actions, funscript.Actions)
	sort.SliceStable(actions, func(i, j int) bool { return funscript.Actions[i].Pos < funscript.Actions[j].Pos })

	if actions[0].At != (136740671 % int64(len(actions))) {
		return false
	}

	for i := range actions {
		if i == 0 {
			continue
		}
		if actions[i].Pos != actions[i-1].Pos+1 {
			return false
		}
	}
	return true
}

func (funscript Script) getDuration() float64 {
	maxts := funscript.Actions[len(funscript.Actions)-1].At
	duration := float64(maxts) / 1000.0

	if funscript.Metadata != nil {
		metadataDuration := float64(funscript.Metadata.Duration)

		if metadataDuration > 50000 {
			// large values are likely in milliseconds
			metadataDuration = metadataDuration / 1000.0
		}
		if metadataDuration > duration {
			duration = metadataDuration
		}
	}
	return duration
}

func getFunscriptDuration(path string) (float64, error) {
	if !strings.HasSuffix(path, ".funscript") {
		return 0.0, fmt.Errorf("not a funscript: %s", path)
	}

	funscript, err := LoadFunscriptData(path)
	if err != nil {
		return 0.0, err
	}
	if funscript.IsFunscriptToken() {
		return 0.0, fmt.Errorf("funscript is a token: %s", path)
	}

	return funscript.getDuration(), nil
}

func (funscript Script) CalculateAPM() int {
	n := len(funscript.Actions)
	if n < 2 {
		return 0
	}

	if funscript.IsFunscriptToken() {
		return 0
	}

	sort.SliceStable(funscript.Actions, func(i, j int) bool { return funscript.Actions[i].At < funscript.Actions[j].At })

	tStart := funscript.Actions[0].At
	tEnd := funscript.Actions[n-1].At
	durationMs := tEnd - tStart
	if durationMs <= 0 {
		return 0
	}

	apm := math.Round(float64(n) * 60000.0 / float64(durationMs))
	return int(apm)
}

func CalculateFunscriptAPM(path string) (int, error) {
	if !strings.HasSuffix(path, ".funscript") {
		return 0, fmt.Errorf("not a funscript: %s", path)
	}

	funscript, err := LoadFunscriptData(path)
	if err != nil {
		return 0, err
	}
	if funscript.IsFunscriptToken() {
		return 0, fmt.Errorf("funscript is a token: %s", path)
	}

	return funscript.CalculateAPM(), nil
}

// CalculateSpeed calculates the average physical movement speed in units/second (0-100 position scale per second),
// as used by FapTap, funscript.io and Eroscripts tools.
// Idle gaps and stationary intervals are filtered out so that long pauses do not skew the active movement speed.
func (funscript Script) CalculateSpeed() int {
	n := len(funscript.Actions)
	if n < 2 {
		return 0
	}

	if funscript.IsFunscriptToken() {
		return 0
	}

	sort.SliceStable(funscript.Actions, func(i, j int) bool { return funscript.Actions[i].At < funscript.Actions[j].At })

	var totalDistance float64
	var movingDurationMs int64

	for i := 1; i < n; i++ {
		dt := funscript.Actions[i].At - funscript.Actions[i-1].At
		if dt <= 0 {
			continue
		}

		dp := math.Abs(float64(funscript.Actions[i].Pos - funscript.Actions[i-1].Pos))
		totalDistance += dp

		segSpeed := (dp * 1000.0) / float64(dt)
		// Active moving segment: position changed and not an idle pause (> 3s with negligible speed < 5 units/s)
		if dp > 0 && !(dt > 3000 && segSpeed < 5.0) {
			movingDurationMs += dt
		}
	}

	if movingDurationMs <= 0 {
		tStart := funscript.Actions[0].At
		tEnd := funscript.Actions[n-1].At
		totalDurationMs := tEnd - tStart
		if totalDurationMs > 0 && totalDistance > 0 {
			return int(math.Round(totalDistance * 1000.0 / float64(totalDurationMs)))
		}
		return 0
	}

	speed := math.Round(totalDistance * 1000.0 / float64(movingDurationMs))
	return int(speed)
}

func CalculateFunscriptSpeed(path string) (int, error) {
	if !strings.HasSuffix(path, ".funscript") {
		return 0, fmt.Errorf("not a funscript: %s", path)
	}

	funscript, err := LoadFunscriptData(path)
	if err != nil {
		return 0, err
	}
	if funscript.IsFunscriptToken() {
		return 0, fmt.Errorf("funscript is a token: %s", path)
	}

	return funscript.CalculateSpeed(), nil
}
