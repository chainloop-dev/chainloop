package attribution

import (
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
)

// Enrich adds AI/human attribution to code changes based on AI line range data.
// The sessionID identifies which session produced the changes, and aiFiles maps
// file paths to the line ranges modified by the AI in that session.
func Enrich(sessionID string, aiFiles map[string][]aicodingsession.LineRange, changes *aicodingsession.CodeChanges) {
	if len(aiFiles) == 0 {
		// No AI line data — all changes are human
		changes.HumanLinesAdded = changes.LinesAdded
		changes.HumanLinesRemoved = changes.LinesRemoved
		for i := range changes.Files {
			changes.Files[i].Attribution = trace.AttributionHuman
		}

		return
	}

	for i := range changes.Files {
		f := &changes.Files[i]
		// An empty ranges slice is valid: the agent only deleted lines.
		if aiRanges, ok := aiFiles[f.Path]; ok {
			f.Attribution = trace.AttributionAI
			f.LineRanges = aiRanges
			f.SessionIDs = []string{sessionID}
			// Count AI lines as the lines covered by AI ranges, capped at the file's diff totals
			aiLineCount := 0
			for _, r := range aiRanges {
				aiLineCount += r.End - r.Start + 1
			}
			aiAdded := min(aiLineCount, f.LinesAdded)
			changes.AILinesAdded += aiAdded
			changes.HumanLinesAdded += f.LinesAdded - aiAdded
			changes.AILinesRemoved += f.LinesRemoved
		} else {
			f.Attribution = trace.AttributionHuman
			changes.HumanLinesAdded += f.LinesAdded
			changes.HumanLinesRemoved += f.LinesRemoved
		}
	}
}
