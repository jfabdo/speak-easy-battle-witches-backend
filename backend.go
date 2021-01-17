package main

import (
	"math"
	"time"

	"evilrus.c/speak-easy/toolbox"
	"github.com/mediocregopher/radix/v3"
)

type Coords struct {
	when time.Time
	x    float64
	y    float64
}

var (
	conn        = toolbox.GetPool()
	gameWidth   = 40
	gameHeight  = 12
	paddleLen   = 6
	startCoords = Coords{time.Time{}, float64(gameWidth) / 2, float64(gameHeight) / 2}
	speed       = 5
	angle       = math.Acos(float64(gameHeight/gameWidth)) * math.Pi //radians
	gameLobby   = "games-pgpg"
	gameStats   = gameLobby + "-stats"
	gameScores  = "game-scores"
)

func main() {
	for true {
		games := make(chan map[string]int)
		conn.Do(radix.Cmd(games, "hget", gameLobby))
		goingGames := <-games
		for k, v := range goingGames {
			if v == 0 {
				go runGame(k)
			}
		}
	}
}

func runGame(gameNum string) {
	checkSet := make(chan []string)
	conn.Do(radix.Cmd(nil, "hset", gameLobby, gameNum, "1"))
	conn.Do(radix.Cmd(checkSet, "lrange", gameLobby+"-"+gameNum, 0, -1))
	if <-checkSet != nil {
		print("Game runner already exists for game " + gameNum)
		return
	}
	var coords []Coords
	append(coords, startCoords)
	expectationsMet := true
	startTime := time.Now()
	for expectationsMet {
		//how is time being handled?
		startTime = startTime.Add(100 * time.Microsecond)
		nextCoords, expectation := calculateNext(coords[len(coords)-1].x, coords[len(coords)-1].y, angle, speed)
		nextCoords.when = startTime
		append(coords, nextCoords)
		go conn.Do(radix.Cmd(nil, "rpush", gameStats+gameNum, coords))
		if expectation != nil {
			conn.Do(radix.Cmd(nil, "rpush", gameStats+gameNum, expectation))
			expectationsMet = checkExpection(expectation)
		}
	}

	go conn.Do(radix.Cmd(nil, "del", gameLobby, gameNum))
	go conn.Do(radix.Cmd(nil, "del", gameLobby+"-"+gameNum))
	go conn.Do(radix.Cmd(nil, "hset", gameScores, gameNum+"-win"))  //, win player))
	go conn.Do(radix.Cmd(nil, "hset", gameScores, gameNum+"-lose")) //,lose player))

}

func calculateNext(x float64, y float64, ballAngle float64, speed int) (Coords, Coords) { //finish this when we're awake
	x1 := math.Cos(ballAngle) * .1 * float64((speed * 2))
	y1 := math.Sin(ballAngle) * .1 * float64((speed * 2))
	var expectation Coords

	if y1 <= 0 {
		y1 *= -1
		angle -= math.Pi
	} else if int(y1) >= gameHeight {
		y1 -= float64(gameHeight * 2)
	}

	if x1 <= 0 {
		var m = (y1 - y) / (x1 - x)
		expectation = Coords{time.Time{}, m*x1 + y1, 0}
	} else if int(x1) >= gameWidth {
		m := (y1 - y) / (x1 - x)
		expectation = Coords{time.Time{}, m*x1 + float64(gameWidth) + y1, float64(gameWidth)}
	}

	return Coords{time.Time{}, x1, y1}, expectation
}

func sleepUntil(t time.Time) {
	<-time.NewTimer(t.Sub(time.Now())).C
}

func checkExpection(expct Coords) bool {
	var player int
	if expct.x == 0 {
		player := 1
	} else {
		player := 2
	}
	location := make(chan float64)
	sleepUntil(expct.when)
	conn.Do(radix.Cmd(location, "hget", gameStats+gameNum, string(player)+"-paddle-location"))
	paddleLoc := <-location
	if paddleLoc < expct.x && paddleLoc+float64(paddleLen) > expct.x {
		return true
	}
	return false
}
