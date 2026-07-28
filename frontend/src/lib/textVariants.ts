export type TextSegment = { text: string; start: number; end: number; wordLike: boolean };
export type VariantContext = { targetContext: string; markedTargetContext: string };

const selectionStartMarker = "<alt-selection>";
const selectionEndMarker = "</alt-selection>";

export function tokenizeTranslation(text: string, language: string): TextSegment[] {
  if (typeof Intl.Segmenter === "function") {
    const segmenter = new Intl.Segmenter(language || undefined, { granularity: "word" });
    return Array.from(segmenter.segment(text), (segment) => ({ text: segment.segment, start: segment.index, end: segment.index + segment.segment.length, wordLike: Boolean(segment.isWordLike) }));
  }
  return fallbackSegments(text);
}

export function surroundingContext(text: string, language: string, selected: TextSegment, distance = 12): string {
	return variantContext(text, language, selected, distance).targetContext;
}

// variantContext marks the exact clicked occurrence, including the first word
// in a sentence, so repeated terms cannot make a variant request ambiguous.
export function variantContext(text: string, language: string, selected: TextSegment, distance = 12): VariantContext {
	const segments = tokenizeTranslation(text, language);
	const words = segments.filter((segment) => segment.wordLike);
	const index = words.findIndex((segment) => segment.start === selected.start && segment.end === selected.end);
	if (index < 0) return { targetContext: selected.text, markedTargetContext: `${selectionStartMarker}${selected.text}${selectionEndMarker}` };
	const first = words[Math.max(0, index - distance)];
	const last = words[Math.min(words.length - 1, index + distance)];
	const lastSegmentIndex = segments.findIndex((segment) => segment.start === last.start && segment.end === last.end);
	let contextEnd = last.end;
	for (let segmentIndex = lastSegmentIndex + 1; segmentIndex < segments.length && !segments[segmentIndex].wordLike; segmentIndex += 1) contextEnd = segments[segmentIndex].end;
	const targetContext = text.slice(first.start, contextEnd);
	const selectionStart = selected.start - first.start;
	const markedTargetContext = `${targetContext.slice(0, selectionStart)}${selectionStartMarker}${selected.text}${selectionEndMarker}${targetContext.slice(selectionStart + selected.text.length)}`;
	return { targetContext, markedTargetContext };
}

export function locateReplacement(text: string, selected: TextSegment, target: string): { start: number; end: number } | null {
  if (!target) return null;
  let index = text.indexOf(target);
  while (index >= 0) {
    const end = index + target.length;
    if (index <= selected.start && end >= selected.end) return { start: index, end };
    index = text.indexOf(target, index + 1);
  }
  return null;
}

function fallbackSegments(text: string): TextSegment[] {
  const expression = /[\p{L}\p{N}\p{M}]+|[^\p{L}\p{N}\p{M}]+/gu;
  const segments: TextSegment[] = [];
  for (const match of text.matchAll(expression)) {
    const value = match[0];
    const start = match.index ?? 0;
    segments.push({ text: value, start, end: start + value.length, wordLike: /^[\p{L}\p{N}\p{M}]+$/u.test(value) });
  }
  return segments;
}
