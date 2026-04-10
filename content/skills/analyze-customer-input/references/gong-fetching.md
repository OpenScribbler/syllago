# Gong Call Fetching Protocol

This reference defines the MCP tool sequence for fetching Gong call transcripts. Load this when the source content is Gong calls, before proceeding to Phase 1b of the AnalyzeSession workflow.

---

## Tool Sequence

Fetch Gong calls in this order. Do not begin extraction until all calls have been fetched.

### Step 1: Search for calls

Use `mcp__gong__search_calls` with:
- `title_filter` -- the customer name or search term provided by the analyst
- Date range -- computed from the "days back" parameter (default 180 days if not specified)

### Step 2: Check pagination

Read `total_pages` from the search response. If `total_pages > 1`, fetch every page before proceeding. Log the total call count:

```
Found [N] calls across [total_pages] pages for "[title_filter]"
```

Do not proceed to transcript fetching until all pages have been consumed. Missing a page means missing calls, which means incomplete analysis.

### Step 3: Get call overviews

For each call returned by the search, use `mcp__gong__get_call_overview` to retrieve call metadata (date, duration, participants). This provides the participants table needed for Phase 2 (affiliation mapping).

### Step 4: Get full transcripts

For each call, use `mcp__gong__get_transcript` to retrieve the full transcript text. Retain the callId, date, and participants alongside the transcript -- these are needed for evidence attribution in Phase 3.

---

## Output

After completing all four steps, you should have for each call:
- **callId** -- unique identifier, used as `source` in extracted items
- **date** -- call date, used for temporal relevance classification
- **participants** -- name, affiliation, and role for each participant
- **transcript** -- full transcript text

These become the `source_content` inputs for AnalyzeSession Phase 2.

---

## Error Handling

| Situation | Action |
|-----------|--------|
| Search returns zero calls | Ask the analyst to verify the search term and date range. Do not conclude there is nothing to analyze without confirmation. |
| Search returns calls but a transcript fetch fails | Report the specific callId that failed. Ask the analyst whether to proceed without it or retry. |
| Pagination returns fewer calls than expected | Log the discrepancy and ask the analyst if the count looks right before proceeding. |
| title_filter not provided | Ask the analyst for the customer name or search term to use. |
