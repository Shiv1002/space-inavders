package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image/color"
	"log"
	"math"
	"math/rand/v2"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/gofont/goregular"
)

// 1.  A controllable player
// 2. bakground for space (stars )
const sampleRate = 8000

type Stars struct {
	X, Y float32
}

type Player struct {
	X, Y                   float32
	speed                  int
	scale                  int
	planeSizeX, planeSizeY int
	playerScale            int
	score                  int
	isFiring               bool
}

type Alien struct {
	X, Y   float32
	color  string
	size   int
	speed  int
	health byte
	dying  bool
	fade   float32 // 1.0 -> fully visible, 0.0 -> invisible
}

const starRadius = 2.0

type Game struct {
	isStarted       bool
	restart         bool
	frontStars      []Stars
	backStars       []Stars
	player          Player
	aliens          []Alien
	missile         []Stars
	audioContext    *audio.Context
	backGroundMusic *audio.Player
	shootSound      *audio.Player
	collisionSound  *audio.Player
	alienDiesSound  *audio.Player
	planeImage      *ebiten.Image // Cached plane image
}

const width = 680
const height = 480
const maxAliensCount = 5

var planePixels = []string{
	"     GG     ", // Nose
	"     GG     ",
	"    GGGG    ",
	"    GccG    ", // Cockpit
	"    GccG    ",
	"   GGGGGG   ",
	"  GGGGGGGG  ",
	" GGGGGGGGGG ",
	"GGGGGGGGGGGG", // Wings start
	"GGGGDDDDGGGG", // Wings sweep
	"GGGD    DGGG",
	" GGD    DGG ",
	"  GG    GG  ",
	"   G RR G   ", // Engines
	"   G RR G   ",
	"    G  G    ", // Tail fins
}
var alienPixels = []string{
	"      GGGG      ",
	"    GGGGGGGG    ",
	"   GGGGGGGGGG   ",
	"  GGGGGGGGGGGG  ",
	" GGGGGGGGGGGGGG ",
	" GGBBGGGGGGBBGG ", // Start of large slanted eyes
	" GBBBBGGGGGBBBBG",
	" GBBBBGGGGGBBBBG",
	"  GBBBGGGGGBBBG ",
	"  GGBBGGGGGBBGG ",
	"   GGGGGGGGGG   ",
	"    GGGGGGGG    ",
	"     GGSSGG     ", // Chin/Shadow
	"      SSSS      ",
	"       SS       ",
	"       SS       ", // Neck
}

func (g *Game) Update() error {
	player := &g.player
	if ebiten.IsKeyPressed(ebiten.KeyW) || ebiten.IsKeyPressed(ebiten.KeyUp) {
		if player.Y-float32(player.planeSizeY) < 0 {
			return nil
		}
		player.Y -= float32(player.speed)

	}
	if ebiten.IsKeyPressed(ebiten.KeyS) || ebiten.IsKeyPressed(ebiten.KeyDown) {
		if player.Y+float32(player.planeSizeY) > height {
			return nil
		}
		player.Y += float32(player.speed)

	}
	if ebiten.IsKeyPressed(ebiten.KeyA) || ebiten.IsKeyPressed(ebiten.KeyLeft) {
		if player.X-float32(player.planeSizeX) < 0 {
			return nil
		}
		g.player.X -= float32(player.speed)

	}

	if ebiten.IsKeyPressed(ebiten.KeyD) || ebiten.IsKeyPressed(ebiten.KeyRight) {
		if player.X+float32(player.planeSizeX) > width {
			return nil
		}
		g.player.X += float32(player.speed)

	}

	// Touch screen support
	// touchIDs := []ebiten.TouchID{}
	touchIDs := inpututil.JustPressedTouchIDs()
	for _, id := range touchIDs {
		fmt.Println(id)
		tx, ty := ebiten.TouchPosition(id)
		// Click on plane to shoot
		if math.Abs(float64(float32(tx)-player.X)) < 100 && math.Abs(float64(float32(ty)-player.Y)) < 100 {
			g.player.isFiring = true
			g.fireMissile()
			if !g.isStarted {
				g.isStarted = true
				g.restart = false
				g.player.score = 0
			}
		} else if tx < width/2 && player.X-float32(player.planeSizeX) > 0 {
			// Touch left side to move left
			player.X -= float32(player.speed * 5)
		} else if tx >= width/2 && player.X+float32(player.planeSizeX) < width {
			// Touch right side to move right
			player.X += float32(player.speed * 5)
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		// This runs only once until the key is released and pressed again
		g.player.isFiring = true
		g.fireMissile()

		if !g.isStarted {
			g.isStarted = true
			g.restart = false
			g.player.score = 0
		}
	}

	return nil
}

func (g *Game) fireMissile() {
	g.missile = append(g.missile, Stars{X: float32(g.player.X), Y: g.player.Y})
	if g.shootSound != nil && !g.shootSound.IsPlaying() {
		g.shootSound.Rewind()
		g.shootSound.Play()
	}
}

func (g *Game) alienSpeen() int {
	return 1
}

var incr = 0

func (g *Game) restartGame() {
	g.isStarted = false
	g.clearAliens()
}

func (g *Game) clearAliens() {
	g.aliens = []Alien{}
}

func (g *Game) showScore(screen *ebiten.Image) {
	player := &g.player

	textOp := &text.DrawOptions{}
	// textOp.GeoM.Scale(2, 2)
	textOp.GeoM.Translate(width-100, 20)
	// initial position of player
	face := &text.GoTextFace{
		Source: mplusFaceSource,
		Size:   16,
	}
	text.Draw(screen, fmt.Sprintf("Score : %d", player.score), face, textOp)
}

func (g *Game) drawStars(screen *ebiten.Image) {
	// Draw the filled circle
	// The 4th argument 'true' is for anti-aliasing (smoother edges)
	for _, s := range g.frontStars {
		vector.FillCircle(screen, float32(s.X), float32(s.Y), starRadius, color.White, false)
	}
	for _, s := range g.backStars {
		vector.FillCircle(screen, float32(s.X), float32(s.Y), starRadius, color.White, false)
	}
	const frontStarsSpeed = 3.0
	const backStarsSpeed = 1.0

	for i := range g.frontStars {
		g.frontStars[i].Y += frontStarsSpeed
		if g.frontStars[i].Y > height {
			g.frontStars[i].Y = 0
			g.frontStars[i].X = float32(rand.IntN(width))
		}
	}

	for i := range g.backStars {
		g.backStars[i].Y += backStarsSpeed
		if g.backStars[i].Y > height {
			g.backStars[i].Y = 0
			g.backStars[i].X = float32(rand.IntN(width))
		}
	}

	for i := 0; i < len(g.missile); i++ {
		g.missile[i].Y -= float32(g.player.speed + 1)
		if g.missile[i].Y < 0 {
			g.missile = append(g.missile[:i], g.missile[i+1:]...)
		}
	}

	// Limit missiles to prevent performance issues
	if len(g.missile) > 10 {
		g.missile = g.missile[len(g.missile)-10:]
	}
}

func (g *Game) drawPlayerPlane(screen *ebiten.Image) {
	player := &g.player
	// Drawing code goes here
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(g.player.scale), float64(g.player.scale))
	op.GeoM.Translate(float64(player.X)-(3*12), float64(player.Y))
	screen.DrawImage(g.planeImage, op) // Use cached image

	for _, mis := range g.missile {
		// missileOp = 		alienOp := &ebiten.DrawImageOptions{}
		// alienOp.GeoM.Scale(3, 3) // Scale the alien
		vector.FillRect(screen, mis.X-2, mis.Y-10, 6, 12, color.RGBA{R: 255, G: 255, B: 0, A: 255}, false)
	}

}

func (g *Game) showStartScreen(screen *ebiten.Image) {
	player := &g.player
	if g.restart {
		head := &text.DrawOptions{}
		subHead := &text.DrawOptions{}
		// textOp.GeoM.Scale(2, 2)
		head.GeoM.Translate(width/2, height/2-100)
		subHead.GeoM.Translate(width/2, height/2-70)
		// initial position of player
		headFace := &text.GoTextFace{
			Source: mplusFaceSource,
			Size:   24,
		}
		subHeadFace := &text.GoTextFace{
			Source: mplusFaceSource,
			Size:   16,
		}

		text.Draw(screen, fmt.Sprintf("Press Space to Start!!"), headFace, head)
		text.Draw(screen, fmt.Sprintf("Final Score : %d", player.score), subHeadFace, subHead)
	} else {
		// player := &g.player
		txt := "Press SpaceBar to Start!!"

		textOp := &text.DrawOptions{}
		textOp.GeoM.Translate(float64((width/2)-len(txt)*5), height/2)
		// initial position of player
		face := &text.GoTextFace{
			Source: mplusFaceSource,
			Size:   24,
		}
		text.Draw(screen, fmt.Sprintf(txt), face, textOp)
	}
}

// Draw handles rendering
func (g *Game) Draw(screen *ebiten.Image) {
	// player := &g.player

	g.showScore(screen)
	g.drawStars(screen)
	g.drawPlayerPlane(screen)
	if !g.isStarted {
		g.showStartScreen(screen)
		return
	}

	// Draw aliens using vector graphics
	for i := range g.aliens {
		g.drawVectorAlien(screen, g.aliens[i].X, g.aliens[i].Y, 1, g.aliens[i].health, g.aliens[i].fade)
	}
	g.Surf()
	// incr++
	// fmt.Print(incr, "-")
}

func (g *Game) createPlaneImage() *ebiten.Image {
	// Create an 8x6 image
	img := ebiten.NewImage(g.player.planeSizeX, g.player.planeSizeY)

	colors := map[rune]color.RGBA{
		'G': {34, 139, 34, 255},   // Forest Green
		'D': {0, 100, 0, 255},     // Dark Green
		'c': {100, 200, 255, 255}, // Cockpit Glass
		'R': {255, 0, 0, 255},     // Engine Glow
	}

	for y, row := range planePixels {
		for x, char := range row {
			if col, ok := colors[char]; ok {
				img.Set(x, y, col)
			}
		}
	}
	return img
}

func (g *Game) drawVectorAlien(screen *ebiten.Image, x, y, scale float32, health byte, fade float32) {
	// Map health (0-100) to color from red -> green
	healthPct := math.Max(0, math.Min(1, float64(health)/100.0))
	r := uint8((1.0 - healthPct) * 255)
	gcol := uint8(healthPct * 255)
	b := uint8(30)
	// Alpha from fade (0.0-1.0)
	a := uint8(math.Max(0, math.Min(255, float64(fade)*255.0)))

	bodyColor := color.RGBA{R: r, G: gcol, B: b, A: a}
	eyeColor := color.RGBA{R: 0, G: 0, B: 0, A: a}
	eyeGlowColor := color.RGBA{R: 255, G: 80, B: 80, A: a}
	antennaColor := color.RGBA{R: uint8(math.Max(0, float64(r)-20)), G: uint8(math.Max(0, float64(gcol)-40)), B: uint8(math.Max(0, float64(b)-10)), A: a}

	// Main body - ellipse
	vector.FillCircle(screen, x, y, 12*scale, bodyColor, true)
	vector.FillCircle(screen, x, y+5*scale, 10*scale, bodyColor, true)

	// Head dome
	vector.FillCircle(screen, x, y-8*scale, 8*scale, bodyColor, true)

	// Eyes
	eyeOffsetX := 5 * scale
	eyeOffsetY := -2 * scale

	// Left eye
	vector.FillCircle(screen, x-eyeOffsetX, y+eyeOffsetY, 3.5*scale, eyeColor, true)
	vector.FillCircle(screen, x-eyeOffsetX, y+eyeOffsetY, 1.5*scale, eyeGlowColor, true)

	// Right eye
	vector.FillCircle(screen, x+eyeOffsetX, y+eyeOffsetY, 3.5*scale, eyeColor, true)
	vector.FillCircle(screen, x+eyeOffsetX, y+eyeOffsetY, 1.5*scale, eyeGlowColor, true)

	// Antennae
	antennaBaseY := y - 16*scale

	// Left antenna
	vector.StrokeLine(screen, x-3*scale, y-14*scale, x-6*scale, antennaBaseY, 1.5*scale, antennaColor, true)
	vector.FillCircle(screen, x-6*scale, antennaBaseY, 2*scale, antennaColor, true)

	// Right antenna
	vector.StrokeLine(screen, x+3*scale, y-14*scale, x+6*scale, antennaBaseY, 1.5*scale, antennaColor, true)
	vector.FillCircle(screen, x+6*scale, antennaBaseY, 2*scale, antennaColor, true)

	// Arms
	armY := y + 2*scale

	// Left arm
	vector.StrokeLine(screen, x-10*scale, armY, x-16*scale, armY+6*scale, 2*scale, bodyColor, true)
	vector.FillCircle(screen, x-16*scale, armY+6*scale, 2.5*scale, bodyColor, true)

	// Right arm
	vector.StrokeLine(screen, x+10*scale, armY, x+16*scale, armY+6*scale, 2*scale, bodyColor, true)
	vector.FillCircle(screen, x+16*scale, armY+6*scale, 2.5*scale, bodyColor, true)
}

// Layout defines the screen dimensions
func (g *Game) Surf() {
	player := &g.player

	// allAlienXPoints := []float32{}

	// current tick count
	tick := ebiten.Tick()
	if tick%90 == 0 { // Further reduced spawn rate
		spawnNumber := rand.IntN(2) + 1 // Spawn 1-2 aliens max
		for i := 0; i < spawnNumber && len(g.aliens) < maxAliensCount; i++ {
			horizontalSpwanPoint := float32(rand.IntN(width-200) + 100)
			alien := Alien{X: horizontalSpwanPoint, Y: -50, health: 100, dying: false, fade: 1.0}

			// log.Printf("Creating alien at X: %.2f, Y: %.2f\n", alien.X, alien.Y)
			g.aliens = append(g.aliens, alien)
			// log.Printf("Total aliens created: %d\n", len(g.aliens))
		}
	}

	for i := 0; i < len(g.aliens); i++ {
		g.aliens[i].Y += float32(g.alienSpeen())
		if g.aliens[i].Y > height {
			// alien is out of screen
			g.aliens = append(g.aliens[:i], g.aliens[i+1:]...)
		}
	}

	if len(g.aliens) > 0 {
		for i := 0; i < len(g.aliens); i++ {
			a := &g.aliens[i]

			//
			if math.Abs(float64(player.Y-a.Y)) < 10 {
				isCollision := false
				if (player.X > a.X && player.X-a.X < 50) || (player.X < a.X && a.X-player.X < 20) {
					isCollision = true
				}

				if isCollision {
					g.restart = true
					g.collisionSound.Rewind()
					g.collisionSound.Play()
					g.restartGame()
				}
			}
			// check collision between alien and missile
		}
	}

	// Collision detection: missiles vs aliens
	if len(g.aliens) > 0 && len(g.missile) > 0 {
		// iterate in reverse so we can safely remove slices
		for i := len(g.aliens) - 1; i >= 0; i-- {
			for j := len(g.missile) - 1; j >= 0; j-- {
				dx := math.Abs(float64(g.missile[j].X - g.aliens[i].X))
				dy := math.Abs(float64(g.missile[j].Y - g.aliens[i].Y))
				// simple bounding check
				if dx < 24 && dy < 24 && !g.aliens[i].dying {
					// single missile causes death and starts fade-out
					g.aliens[i].health = 0
					g.aliens[i].dying = true
					g.aliens[i].fade = 1.0
					// remove the missile
					g.missile = append(g.missile[:j], g.missile[j+1:]...)
					break
				}
			}

			// process fade-out if dying
			if g.aliens[i].dying {
				g.aliens[i].fade -= 0.05 // fade speed per tick
				if g.aliens[i].fade <= 0 {
					// remove alien
					if g.alienDiesSound != nil {
						g.alienDiesSound.Rewind()
						g.alienDiesSound.Play()
					}
					g.player.score += 1
					g.aliens = append(g.aliens[:i], g.aliens[i+1:]...)

				}
			}
		}
	}
}

// Layout defines the screen dimensions
func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return width, height
}

var (
	mplusFaceSource *text.GoTextFaceSource
)

//go:embed shoot.wav
var shootWav []byte

//go:embed collision.wav
var collisionWav []byte

//go:embed backgroundMusic.wav
var backgroundMusicWav []byte

//go:embed alienDies.wav
var alienDiesWav []byte

func loadSound(audioContext *audio.Context, data []byte) *audio.Player {
	d, err := wav.DecodeWithSampleRate(sampleRate, bytes.NewReader(data))
	if err != nil {
		log.Fatal(err)
	}

	player, err := audioContext.NewPlayer(d)
	if err != nil {
		log.Fatal(err)
	}
	return player
}

func loadLoopingMusic(audioContext *audio.Context, data []byte) *audio.Player {
	d, err := wav.DecodeWithSampleRate(sampleRate, bytes.NewReader(data))
	if err != nil {
		log.Fatal(err)
	}

	loopStream := audio.NewInfiniteLoop(d, d.Length())

	player, err := audioContext.NewPlayer(loopStream)
	if err != nil {
		log.Fatal(err)
	}
	return player
}

func main() {
	s, _ := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
	audioContext := audio.NewContext(sampleRate)
	mplusFaceSource = s

	game := &Game{}

	game.audioContext = audioContext
	ebiten.SetWindowSize(width, height)
	ebiten.SetWindowTitle("My Pixel Plane Game")

	ebiten.SetTPS(30)
	game.shootSound = loadSound(audioContext, shootWav)
	game.collisionSound = loadSound(audioContext, collisionWav)
	game.alienDiesSound = loadSound(audioContext, alienDiesWav)
	game.backGroundMusic = loadLoopingMusic(audioContext, backgroundMusicWav)

	game.backGroundMusic.SetVolume(0.3)
	game.backGroundMusic.Play()
	game.collisionSound.SetVolume(100) // Fixed: was 100.0
	game.alienDiesSound.SetVolume(0.8)

	game.player.X = float32(width / 2)
	game.player.Y = float32(height * 0.8)
	game.player.scale = 4
	game.player.planeSizeX = 12
	game.player.planeSizeY = 16
	game.player.speed = 6
	game.player.score = 0

	// Cache the plane image (created once, not every frame)
	game.planeImage = game.createPlaneImage()

	// Drastically reduced star count for WebAssembly performance
	for x := 0; x <= width; x += 30 {
		for y := 0; y <= height; y += 30 {
			if showStar := rand.IntN(20); showStar == 1 {
				game.frontStars = append(game.frontStars, Stars{X: float32(x), Y: float32(y)})
			}
		}
	}

	for x := 0; x <= width; x += 100 {
		for y := 0; y <= height; y += 100 {
			if showStar := rand.IntN(10); showStar == 1 {
				game.backStars = append(game.backStars, Stars{X: float32(x), Y: float32(y)})
			}
		}
	}

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
