package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/golang/freetype/truetype"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/colornames"
	"golang.org/x/image/font"
	_ "golang.org/x/image/font"
	_ "image"
	_ "image/png"
	"io/fs"
	"io/ioutil"
	"log"
	"math"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"time"
)

const metaFileName = "meta.json"
const gamesRootPath = "./games"
const killHoldDuration = 60
const navigationTimeout = 20
const deadZone = .5
const menuFontSize = 32

//go:embed Roboto-Regular.ttf
var robotoRegular []byte
var menuFontFace font.Face

func init() {
	f, err := truetype.Parse(robotoRegular)
	if err != nil {
		log.Print(err)
		return
	}
	options := &truetype.Options{
		Size:    menuFontSize,
		Hinting: font.HintingFull,
	}
	menuFontFace = truetype.NewFace(f, options)
}

type GameMeta struct {
	Name           string    `json:"name"`
	Author         string    `json:"author"`
	ReleaseDate    time.Time `json:"release_date"`
	ThumbnailPath  string    `json:"thumbnail_path"`
	ExecutablePath string    `json:"executable_path"`
	path           string
}

type Game struct {
	Name           string
	Author         string
	ReleaseDate    time.Time
	Thumbnail      *ebiten.Image
	ExecutablePath string
	animation      float64
}

type App struct {
	games         []Game
	selectedIndex int
	currentChild  *exec.Cmd
	slide         float64
	timeout       float64
}

func (g *App) Update() error {
	if g.currentChild == nil {
		g.navigation()
		g.animations()
	}
	g.openAndClosing()
	return nil
}

func (g *App) navigation() {
	if g.timeout < 0 {
		if ebiten.GamepadAxis(0, 0) > deadZone {
			g.selectedIndex++
			g.timeout = navigationTimeout
		}
		if ebiten.GamepadAxis(0, 0) < -deadZone {
			g.selectedIndex--
			g.timeout = navigationTimeout
		}
	}
	g.selectedIndex %= len(g.games)
	if g.selectedIndex < 0 {
		g.selectedIndex = len(g.games) - 1
	}
	g.timeout--
}

func (g *App) openAndClosing() {
	if g.currentChild == nil {
		if inpututil.IsGamepadButtonJustPressed(0, ebiten.GamepadButton0) {
			s := g.games[g.selectedIndex%len(g.games)]
			go g.wait(&s)
		}
	} else {
		a := inpututil.GamepadButtonPressDuration(0, ebiten.GamepadButton7) > killHoldDuration
		b := inpututil.GamepadButtonPressDuration(0, ebiten.GamepadButton6) > killHoldDuration
		if a && b {
			err := g.currentChild.Process.Kill()
			if err != nil {
				log.Println(err)
				return
			}
		}
	}
}

func (g *App) animations() {
	for i := range g.games {
		if i == g.selectedIndex {
			g.games[i].animation = lerp(g.games[i].animation, 1, .2)
		} else {
			g.games[i].animation = lerp(g.games[i].animation, 0, .2)
		}
	}
}

func lerp(a, b, t float64) float64 {
	return (b-a)*t + a
}

func (g *App) wait(game *Game) {
	g.currentChild = exec.Command(game.ExecutablePath)
	err := g.currentChild.Start()
	if err != nil {
		log.Println(err)
		return
	}
	fmt.Println("started")
	err = g.currentChild.Wait()
	if err != nil {
		log.Println(err)
		return
	}
	fmt.Println("exited")
	g.currentChild = nil
	ebiten.RestoreWindow()
}

func (g *App) Draw(screen *ebiten.Image) {
	width, height := ebiten.WindowSize()
	minSize := math.Min(float64(width), float64(height))
	thumbnailSize := float64(minSize) * .8

	g.slide = lerp(g.slide, float64(g.selectedIndex), .2)

	for x, game := range g.games {
		size := game.Thumbnail.Bounds().Size()

		options := &ebiten.DrawImageOptions{}
		options.Filter = ebiten.FilterLinear
		colorScale := remap(game.animation, 0, 1, .5, 1)
		options.ColorM.Scale(colorScale, colorScale, colorScale, 1)

		options.GeoM.Scale(1/float64(size.X), 1/float64(size.Y))
		// is now 0-1
		options.GeoM.Translate(-.5, -.5)
		scale := remap(game.animation, 0, 1, .8, 1)
		options.GeoM.Scale(scale, scale)
		options.GeoM.Translate(float64(x)-g.slide, 0)
		options.GeoM.Scale(thumbnailSize, thumbnailSize)
		// is now screen space
		options.GeoM.Translate(float64(width/2), float64(height/2))
		screen.DrawImage(game.Thumbnail, options)
	}

	name := fmt.Sprintf("%v by %v", g.games[g.selectedIndex].Name, g.games[g.selectedIndex].Author)
	rect := text.BoundString(menuFontFace, name)
	size := rect.Size()
	text.Draw(screen, name, menuFontFace, width/2-size.X/2, height-size.Y, colornames.White)
}

func remap(t, fromA, toA, fromB, toB float64) float64 {
	normalized := (t - fromA) / (toA - fromA)
	return normalized*(toB-fromB) + fromB
}

func (g *App) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return outsideWidth, outsideHeight
}

func main() {
	w, h := ebiten.ScreenSizeInFullscreen()
	ebiten.SetWindowSize(w, h)
	ebiten.SetFullscreen(true)
	ebiten.SetWindowTitle("Acagamics Arcade Launcher")
	ebiten.SetWindowResizable(false)
	ebiten.SetCursorMode(ebiten.CursorModeHidden)
	metas := loadGameMetas()
	games := loadGames(metas)
	fmt.Println(games)
	if err := ebiten.RunGame(&App{games: games}); err != nil {
		log.Fatal(err)
	}
}

func loadGameMetas() []GameMeta {
	games := make([]GameMeta, 0)
	err := filepath.Walk(gamesRootPath, func(path string, info fs.FileInfo, err error) error {
		if info.Name() == metaFileName && !info.IsDir() {
			game, err := loadGameMeta(path)
			if err != nil {
				log.Println(err)
				return err
			}
			games = append(games, game)
			fmt.Printf("found game '%v' at %v\n", game.Name, path)
		}
		return nil
	})
	if err != nil {
		log.Fatalln(err)
	}
	return games
}

func loadGameMeta(path string) (GameMeta, error) {
	file, err := os.Open(path)
	if err != nil {
		log.Printf("could not open %v\n", path)
		return GameMeta{}, err
	}
	defer file.Close()

	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		log.Printf("could not read %v\n", path)
		return GameMeta{}, err
	}
	var game GameMeta
	err = json.Unmarshal(bytes, &game)
	if err != nil {
		log.Printf("could not unmarshal %v\n", path)
		return GameMeta{}, err
	}
	game.path = filepath.Dir(path)
	return game, nil
}

func loadGames(metas []GameMeta) []Game {
	games := make([]Game, 0)
	for _, foo := range metas {
		game, err := loadGame(foo)
		if err != nil {
			log.Printf("could not load game '%v'\n", game.Name)
			log.Println(err)
			continue
		}
		games = append(games, game)
	}
	return games
}

func loadGame(meta GameMeta) (Game, error) {
	file, _, err := ebitenutil.NewImageFromFile(path.Join(meta.path, meta.ThumbnailPath))
	p := meta.ExecutablePath
	if !path.IsAbs(meta.ExecutablePath) {
		p = path.Join(meta.path, meta.ExecutablePath)
	}
	if err != nil {
		return Game{}, err
	}
	return Game{
		Name:           meta.Name,
		Author:         meta.Author,
		ReleaseDate:    meta.ReleaseDate,
		ExecutablePath: p,
		Thumbnail:      cutThumbnail(file),
		animation:      1,
	}, nil
}

func cutThumbnail(image *ebiten.Image) *ebiten.Image {
	bounds := image.Bounds()
	size := bounds.Size()
	if size.X > size.Y {
		sub := size.X - size.Y
		bounds.Min.X += sub / 2
		bounds.Max.X -= sub / 2
	} else {
		sub := size.Y - size.X
		bounds.Min.Y += sub / 2
		bounds.Max.Y -= sub / 2
	}
	return image.SubImage(bounds).(*ebiten.Image)
}
