package core

import (
	"errors"
	"fmt"
	"time"
)

const Version = "0.1.0"

type ActionType int8

const (
	Attack ActionType = iota
	Defence
)

type ActionLevel int8

func (l ActionLevel) Sub(level ActionLevel) int8 {
	return int8(l - level)
}

type Action struct {
	Type  ActionType
	Level ActionLevel
}

type ActionList []Action

func (al ActionList) Remove(action Action) (ActionList, bool) {
	for i, a := range al {
		if a == action {
			return append(al[:i], al[i+1:]...), true
		}
	}
	return al, false
}

func (al ActionList) Clone() ActionList {
	return append(al[:0:0], al...)
}

const InfiniteThinkingTime time.Duration = 0

type PlayerID uint32

type Player struct {
	ID   PlayerID `json:"id"`
	Name string   `json:"name"`
}

type PlayerSet []*Player

func (ps PlayerSet) Get(id PlayerID) (*Player, bool) {
	for _, p := range ps {
		if p.ID == id {
			return p, true
		}
	}
	return nil, false
}

type GameSettings struct {
	Version               string        `json:"version"`
	Players               PlayerSet     `json:"players"`
	TotalGames            uint32        `json:"TotalGames"`
	InitialThinkingTime   time.Duration `json:"initialThinkingTime"`
	ThinkingTimeIncrement time.Duration `json:"thinkingTimeIncrement"`
	Actions               ActionList    `json:"actions"`
	JustGuardPoint        int32         `json:"justGuardPoint"`
}

type PlayerState struct {
	PlayerID PlayerID `json:"playerId"`
	// Current points.
	Points int32 `json:"points"`
	// Remaining thinking time.
	ThinkingTime time.Duration `json:"thinkingTime"`
	// Available actions.
	Actions ActionList `json:"actions"`
}

func (s *PlayerState) Clone() *PlayerState {
	return &PlayerState{
		PlayerID:     s.PlayerID,
		Points:       s.Points,
		ThinkingTime: s.ThinkingTime,
		Actions:      s.Actions.Clone(),
	}
}

type PlayerStateSet []*PlayerState

func (s PlayerStateSet) Get(id PlayerID) (*PlayerState, bool) {
	for _, ps := range s {
		if ps.PlayerID == id {
			return ps, true
		}
	}
	return nil, false
}

func (s PlayerStateSet) Clone() PlayerStateSet {
	r := make(PlayerStateSet, 0, len(s))
	for _, ps := range s {
		r = append(r, ps.Clone())
	}
	return r
}

type PlayerAction struct {
	PlayerID                PlayerID      `json:"playerId"`
	TargetPlayerID          PlayerID      `json:"targetPlayerId"`
	Action                  Action        `json:"action"`
	ThinkingTimeConsumption time.Duration `json:"thinkingTimeConsumption"`
}

type PlayerActionSet []*PlayerAction

func (pas PlayerActionSet) Get(id PlayerID) (*PlayerAction, bool) {
	for _, pa := range pas {
		if pa.PlayerID == id {
			return pa, true
		}
	}
	return nil, false
}

const GameOver uint32 = 0

type GameState struct {
	GameNum      uint32         `json:"gameNum"`
	PlayerStates PlayerStateSet `json:"playerStates"`
}

func NewGameState(settings *GameSettings) *GameState {
	pss := make(PlayerStateSet, 0, len(settings.Players))
	for _, p := range settings.Players {
		pss = append(pss, &PlayerState{
			PlayerID:     p.ID,
			Points:       0,
			ThinkingTime: settings.InitialThinkingTime,
			Actions:      settings.Actions.Clone(),
		})
	}
	return &GameState{
		GameNum:      1,
		PlayerStates: pss,
	}
}

func (s *GameState) Clone() *GameState {
	return &GameState{
		GameNum:      s.GameNum,
		PlayerStates: s.PlayerStates.Clone(),
	}
}

type Game struct {
	Settings   *GameSettings     `json:"settings"`
	ActionLogs []PlayerActionSet `json:"actionLogs"`
	State      *GameState        `json:"state"`
}

func NewGame(settings *GameSettings) *Game {
	return &Game{
		Settings:   settings,
		ActionLogs: make([]PlayerActionSet, 0),
		State:      NewGameState(settings),
	}
}

// ApplyPlayerAction will mutate ActionLogs and State.
func (g *Game) ApplyPlayerAction(playerActions PlayerActionSet) error {
	if len(g.Settings.Players) != len(playerActions) {
		return errors.New("invalid size of player action set")
	}
	if g.State.GameNum == GameOver {
		return errors.New("game was over")
	}
	state := g.State.Clone()
	for _, pa := range playerActions {
		ps, found := state.PlayerStates.Get(pa.PlayerID)
		if !found {
			return fmt.Errorf("player (id: %d) state not found", pa.PlayerID)
		}
		// Update `ps.Points`.
		switch pa.Action.Type {
		case Attack:
			tpa, found := playerActions.Get(pa.TargetPlayerID)
			if !found {
				return fmt.Errorf("player (id: %d) action not found", pa.TargetPlayerID)
			}
			switch tpa.Action.Type {
			case Defence:
				points := pa.Action.Level.Sub(tpa.Action.Level)
				if points > 0 {
					ps.Points += int32(points)
				} else if points == 0 {
					tps, found := state.PlayerStates.Get(pa.TargetPlayerID)
					if !found {
						return fmt.Errorf("player (id: %d) state not found", pa.PlayerID)
					}
					tps.Points += g.Settings.JustGuardPoint
				}
			default:
				ps.Points += int32(pa.Action.Level)
			}
		default:
			// do nothing
		}
		// Update `ps.Actions`.
		{
			as, ok := ps.Actions.Remove(pa.Action)
			if !ok {
				return errors.New("unavailable action")
			}
			if len(as) == 0 {
				state.GameNum++
				if state.GameNum > g.Settings.TotalGames {
					state.GameNum = GameOver
				} else {
					ps.Actions = g.Settings.Actions.Clone()
				}
			} else {
				ps.Actions = as
			}
		}
		// Update `ps.ThinkingTime`.
		{
			if ps.ThinkingTime < pa.ThinkingTimeConsumption {
				return errors.New("over thinking time")
			}
			ps.ThinkingTime -= pa.ThinkingTimeConsumption
			ps.ThinkingTime += g.Settings.ThinkingTimeIncrement
		}
	}
	g.State = state
	g.ActionLogs = append(g.ActionLogs, playerActions)
	return nil
}
