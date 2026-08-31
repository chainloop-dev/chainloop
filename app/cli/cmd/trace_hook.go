package cmd

import (
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/claude"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/config"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/cursor"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/opencode"
	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"
	"github.com/spf13/cobra"
)

func newTraceHookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "hook",
		Short:  "Internal: handle hook invocations",
		Hidden: true,
	}

	cmd.AddCommand(
		newTraceHookGitCmd(),
		newTraceHookClaudeCmd(),
		newTraceHookCursorCmd(),
		newTraceHookOpenCodeCmd(),
	)

	return cmd
}

func newTraceHookGitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "git",
		Short: "Handle git hook invocations",
	}

	cmd.AddCommand(
		newTraceHookGitCommitMsgCmd(),
		newTraceHookGitPostCommitCmd(),
		newTraceHookGitPrePushCmd(),
	)

	return cmd
}

func newTraceHookGitCommitMsgCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "commit-msg [msg-file]",
		Short: "Handle the commit-msg git hook",
		Args:  cobra.ExactArgs(1),
		Annotations: map[string]string{
			"skipActionOptsInit": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cleanup := InitHookLogger()
			defer cleanup()

			return action.HandleCommitMsgHook(cmd.Context(), args[0], logger)
		},
	}
}

func newTraceHookGitPostCommitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "post-commit",
		Short: "Handle the post-commit git hook",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cleanup := InitHookLogger()
			defer cleanup()
			return action.HandlePostCommitHook(cmd.Context(), logger)
		},
	}
}

func newTraceHookGitPrePushCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pre-push",
		Short: "Handle the pre-push git hook",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cleanup := InitHookLogger()
			defer cleanup()

			requireTrace := config.LoadRequireTraceFromYML(".")

			return action.HandlePrePushHook(cmd.Context(), requireTrace, logger, action.RunTracePushOpts{
				ActionOpts: ActionOpts,
				CLIVersion: Version,
			})
		},
	}
}

func newTraceHookClaudeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "claude",
		Short: "Handle Claude Code hook invocations",
	}

	cmd.AddCommand(
		newTraceHookClaudeSessionStartCmd(),
		newTraceHookClaudeSessionEndCmd(),
		newTraceHookClaudePreToolUseCmd(),
		newTraceHookClaudePostToolUseCmd(),
	)

	return cmd
}

func newTraceHookClaudeSessionStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "session-start",
		Short: "Capture session ID on Claude Code session start",
		Annotations: map[string]string{
			"skipActionOptsInit": "true",
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			cleanup := InitHookLogger()
			defer cleanup()
			return action.HandleAgentSessionStart(claude.New(), logger)
		},
	}
}

func newTraceHookClaudeSessionEndCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "session-end",
		Short: "Clean up trace state on Claude Code session end",
		Annotations: map[string]string{
			"skipActionOptsInit": "true",
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			cleanup := InitHookLogger()
			defer cleanup()
			return action.HandleAgentSessionEnd(claude.New(), logger)
		},
	}
}

func newTraceHookClaudePreToolUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pre-tool-use",
		Short: "Snapshot file before Claude Code edit",
		Annotations: map[string]string{
			"skipActionOptsInit": "true",
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			cleanup := InitHookLogger()
			defer cleanup()
			return action.HandleAgentPreToolUse(claude.New(), logger)
		},
	}
}

func newTraceHookClaudePostToolUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "post-tool-use",
		Short: "Record AI line ranges after Claude Code edit",
		Annotations: map[string]string{
			"skipActionOptsInit": "true",
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			cleanup := InitHookLogger()
			defer cleanup()
			return action.HandleAgentPostToolUse(claude.New(), logger)
		},
	}
}

func newTraceHookCursorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cursor",
		Short: "Handle Cursor hook invocations",
	}

	cmd.AddCommand(
		newTraceHookCursorSessionStartCmd(),
		newTraceHookCursorSessionEndCmd(),
		newTraceHookCursorAfterFileEditCmd(),
	)

	return cmd
}

func newTraceHookCursorSessionStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "session-start",
		Short: "Capture session ID on Cursor session start",
		Annotations: map[string]string{
			"skipActionOptsInit": "true",
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			cleanup := InitHookLogger()
			defer cleanup()
			return action.HandleAgentSessionStart(cursor.New(), logger)
		},
	}
}

func newTraceHookCursorSessionEndCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "session-end",
		Short: "Clean up trace state on Cursor session end",
		Annotations: map[string]string{
			"skipActionOptsInit": "true",
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			cleanup := InitHookLogger()
			defer cleanup()
			return action.HandleAgentSessionEnd(cursor.New(), logger)
		},
	}
}

func newTraceHookCursorAfterFileEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "after-file-edit",
		Short: "Record AI line ranges after Cursor file edit",
		Annotations: map[string]string{
			"skipActionOptsInit": "true",
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			cleanup := InitHookLogger()
			defer cleanup()
			return action.HandleAgentPostToolUse(cursor.New(), logger)
		},
	}
}

func newTraceHookOpenCodeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "opencode",
		Short: "Handle opencode hook invocations",
	}

	cmd.AddCommand(
		newTraceHookOpenCodeSessionStartCmd(),
		newTraceHookOpenCodeSessionEndCmd(),
		newTraceHookOpenCodePreToolUseCmd(),
		newTraceHookOpenCodePostToolUseCmd(),
	)

	return cmd
}

func newTraceHookOpenCodeSessionStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "session-start",
		Short: "Capture session ID on opencode session start",
		Annotations: map[string]string{
			"skipActionOptsInit": "true",
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			cleanup := InitHookLogger()
			defer cleanup()
			return action.HandleAgentSessionStart(opencode.New(), logger)
		},
	}
}

func newTraceHookOpenCodeSessionEndCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "session-end",
		Short: "Clean up trace state on opencode session end",
		Annotations: map[string]string{
			"skipActionOptsInit": "true",
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			cleanup := InitHookLogger()
			defer cleanup()
			return action.HandleAgentSessionEnd(opencode.New(), logger)
		},
	}
}

func newTraceHookOpenCodePreToolUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pre-tool-use",
		Short: "Snapshot file before opencode edit",
		Annotations: map[string]string{
			"skipActionOptsInit": "true",
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			cleanup := InitHookLogger()
			defer cleanup()
			return action.HandleAgentPreToolUse(opencode.New(), logger)
		},
	}
}

func newTraceHookOpenCodePostToolUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "post-tool-use",
		Short: "Record AI line ranges after opencode edit",
		Annotations: map[string]string{
			"skipActionOptsInit": "true",
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			cleanup := InitHookLogger()
			defer cleanup()
			return action.HandleAgentPostToolUse(opencode.New(), logger)
		},
	}
}
