package service

import (
	"github.com/DIMO-Network/edge-network/agent"
	"github.com/rs/zerolog"
)

func (app *App) createAgent(logger zerolog.Logger) (agent.Agent1Client, error) {
	a := agent.NewDefaultSimpleAgent(logger)
	return a, nil
}

// Expose app agent on DBus
func (app *App) ExposeAgent(caps string, setAsDefaultAgent bool, logger zerolog.Logger) error {
	return agent.ExposeAgent(app.DBusConn(), app.agent, caps, setAsDefaultAgent, logger)
}
