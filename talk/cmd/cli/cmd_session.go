package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/xvThomas/LLMClientWrapper/talk/internal/domain"
)

func (a *App) cmdMemory(ctx context.Context) {
	sb, ok := a.Store.(domain.SessionBrowser)
	if !ok {
		a.Println(faint("(session history not available)"))
		return
	}
	turns, err := sb.LoadSession(ctx, a.Store.SessionID())
	if err != nil {
		a.Errorf("%s%s\n", red("Error: "), err.Error())
		return
	}
	if len(turns) == 0 {
		a.Println(faint("(no history for current session)"))
		return
	}
	// Resolve session title
	title := ""
	if sessions, err := sb.ListSessions(ctx, a.Store.UserID()); err == nil {
		for _, s := range sessions {
			if s.ID == a.Store.SessionID() {
				title = s.Title
				break
			}
		}
	}
	header := emphasize("Session history:")
	if title != "" {
		header += "  " + cyan(title)
	}
	a.Printf("\n%s  %s\n", header, faint(shortID(a.Store.SessionID())))
	for i, t := range turns {
		turnIDStr := ""
		if t.TurnID != "" {
			turnIDStr = "  " + faint(t.TurnID)
		}
		a.Printf("\n%s  %s%s\n",
			emphasize(fmt.Sprintf("Turn %d", i+1)),
			faint(t.At.Format("2006-01-02 15:04:05")),
			turnIDStr)
		a.Printf("  %s %s\n", green(bold+"You"+reset+":"), t.Question)
		a.Printf("  %s %s\n", cyan(bold+"Assistant"+reset+":"), t.Answer)
		if t.CallCount > 1 {
			a.Printf("  %s\n", faint(fmt.Sprintf("(%d LLM calls)", t.CallCount)))
		}
	}
}

func (a *App) cmdSessions(ctx context.Context) {
	sb, ok := a.Store.(domain.SessionBrowser)
	if !ok {
		a.Println(faint("(sessions not available)"))
		return
	}
	sessions, err := sb.ListSessions(ctx, a.Store.UserID())
	if err != nil {
		a.Errorf("%s%s\n", red("Error: "), err.Error())
		return
	}
	if len(sessions) == 0 {
		a.Println(faint("(no sessions)"))
		return
	}
	a.Printf("\n%s\n", emphasize("Sessions:"))
	for _, s := range sessions {
		marker := ""
		if s.ID == a.Store.SessionID() {
			marker = " " + green("← current")
		}
		title := s.Title
		if title == "" {
			title = "(untitled)"
		}
		a.Printf("  %s  %s  %s  %s%s\n",
			cyan(s.ID),
			title,
			faint(s.CreatedAt.Format("2006-01-02 15:04")),
			faint(fmt.Sprintf("%d turns", s.TurnCount)),
			marker)
	}
	a.Println()
}

func (a *App) cmdSession(ctx context.Context, args string) {
	sb, ok := a.Store.(domain.SessionBrowser)
	if !ok {
		a.Println(faint("(session switching not available)"))
		return
	}

	// /session <id> — switch to an existing session by prefix match
	if args != "" {
		sessions, err := sb.ListSessions(ctx, a.Store.UserID())
		if err != nil {
			a.Errorf("%s%s\n", red("Error: "), err.Error())
			return
		}
		var match string
		for _, s := range sessions {
			if strings.HasPrefix(s.ID, args) {
				match = s.ID
				break
			}
		}
		if match == "" {
			a.Println(yellow("No session found matching: ") + faint(args))
			return
		}
		if err := sb.SetSession(ctx, match); err != nil {
			a.Errorf("%s%s\n", red("Error switching session: "), err.Error())
			return
		}
		a.Printf("Switched to session %s.\n", green(shortID(match)))
		return
	}

	// /session — show sessions and offer to create new or switch
	sessions, err := sb.ListSessions(ctx, a.Store.UserID())
	if err != nil {
		a.Errorf("%s%s\n", red("Error: "), err.Error())
		return
	}
	a.Printf("\n%s  %s\n", emphasize("Sessions:"), faint("user: "+a.Store.UserID()))
	if len(sessions) == 0 {
		a.Println(faint("  (no past sessions found)"))
	} else {
		for i, s := range sessions {
			label := shortID(s.ID)
			marker := ""
			if s.ID == a.Store.SessionID() {
				marker = " " + green("← current")
			}
			turns := ""
			if s.TurnCount > 0 {
				turns = fmt.Sprintf(" (%d turns)", s.TurnCount)
			}
			a.Printf("  [%d] %s  %s%s%s\n", i+1, cyan(label), faint(s.CreatedAt.Format("2006-01-02 15:04")), faint(turns), marker)
		}
	}
	choice, err := a.LR.ReadLine(fmt.Sprintf("Choose [1-%d] or 'new' (Enter to cancel): ", len(sessions)))
	if err != nil || strings.TrimSpace(choice) == "" {
		return
	}
	choice = strings.TrimSpace(choice)
	if choice == "new" {
		newID := domain.GenerateSessionID()
		if err := sb.SetSession(ctx, newID); err != nil {
			a.Errorf("%s%s\n", red("Error creating session: "), err.Error())
			return
		}
		a.Printf("New session created: %s\n", green(shortID(newID)))
		return
	}
	n, err := strconv.Atoi(choice)
	if err != nil || n < 1 || n > len(sessions) {
		a.Println(yellow("Invalid choice."))
		return
	}
	selected := sessions[n-1].ID
	if err := sb.SetSession(ctx, selected); err != nil {
		a.Errorf("%s%s\n", red("Error switching session: "), err.Error())
		return
	}
	a.Printf("Switched to session %s.\n", green(shortID(selected)))
}

// shortID returns a concise display form of a session UUID.
func shortID(id string) string {
	if len(id) > 8 {
		return id[:8] + "…"
	}
	return id
}
