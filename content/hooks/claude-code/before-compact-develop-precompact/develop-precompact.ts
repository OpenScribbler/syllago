#!/usr/bin/env bun
// ~/.config/pai/hooks/develop-precompact.ts
// PreCompact hook: Outputs active workflow status + nextAction before compaction
// This text survives in the compacted summary, enabling recovery after context loss.

import { listActiveWorkflows, formatStageDisplay } from './lib/develop-state';
import type { DevelopState } from './lib/develop-state';

function formatNextAction(state: DevelopState): string {
  if (!state.nextAction) {
    return 'nextAction: null (use determineStartStage() or /develop --resume to re-derive)';
  }

  const na = state.nextAction;
  const lines = [
    `Next Action: ${na.description}`,
    `  tool: ${na.tool}`,
    `  action: ${na.action}`,
  ];

  if (na.toolInput) {
    // Show key fields from toolInput without the full prompt (keep it compact)
    const { prompt, ...rest } = na.toolInput as Record<string, unknown>;
    if (Object.keys(rest).length > 0) {
      lines.push(`  toolInput: ${JSON.stringify(rest)}`);
    }
    if (prompt && typeof prompt === 'string') {
      // Truncate prompt to first 200 chars for compaction summary
      lines.push(`  prompt: "${prompt.slice(0, 200)}${prompt.length > 200 ? '...' : ''}"`);
    }
  }

  if (na.gate) {
    lines.push(`  gate: ${na.gate} (must exist before action)`);
  }

  return lines.join('\n');
}

function formatWorkflow(state: DevelopState): string {
  const lines = [
    `  Feature: ${state.feature}`,
    `  Stage: ${formatStageDisplay(state.currentStage)} (${state.currentStage})`,
    `  ${formatNextAction(state)}`,
  ];

  if (state.artifacts?.design) {
    lines.push(`  Design: ${state.artifacts.design}`);
  }
  if (state.artifacts?.plan) {
    lines.push(`  Plan: ${state.artifacts.plan}`);
  }
  if (state.validation?.attempts && state.validation.attempts > 0) {
    lines.push(`  Validation: ${state.validation.passed ? 'PASSED' : `${state.validation.attempts} attempts, ${state.validation.lastGaps?.length || 0} gaps`}`);
  }
  if (state.beads?.created) {
    lines.push(`  Beads: ${state.beads.taskCount} created (prefix: ${state.beads.prefix})`);
  }

  return lines.join('\n');
}

try {
  const workflows = listActiveWorkflows();

  const parts: string[] = [];

  if (workflows.length > 0) {
    const sections = workflows.map(formatWorkflow);
    const stateFiles = workflows.map(w => {
      const sanitized = w.feature.toLowerCase().replace(/[^a-z0-9-]/g, '-').replace(/-+/g, '-');
      return `.develop/${sanitized}.json`;
    });

    parts.push(`DEVELOP WORKFLOW ACTIVE:

${sections.join('\n\n')}

TO RESUME: Read ${stateFiles.join(', ')}, check nextAction field, execute it.
If nextAction is null, run /develop --resume to re-derive the next step.`);
  }

  if (parts.length > 0) {
    console.log(`<system-reminder>
${parts.join('\n\n')}
</system-reminder>`);
  }

} catch (error) {
  // Never crash - PreCompact hooks must be silent on failure
  console.error('Develop PreCompact hook error:', error);
}

process.exit(0);
