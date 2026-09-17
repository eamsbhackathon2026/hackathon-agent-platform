import { describe, expect, it } from "vitest";

import { onlyInstructionsKept, skillFileName } from "./skill-file-name";

describe("skillFileName", () => {
  it("keeps the name of a kept upload", () => {
    expect(skillFileName({ name: "Review", source_type: "zip", source_filename: "review-pack.zip", source_file_available: true })).toBe("review-pack.zip");
  });

  it("keeps the original Markdown file name", () => {
    expect(skillFileName({ name: "Writing", source_type: "markdown", source_filename: "writing.markdown", source_file_available: false })).toBe("writing.markdown");
  });

  it("names the stored instructions of an older ZIP skill after the package", () => {
    expect(skillFileName({ name: "Review", source_type: "zip", source_filename: "review-pack.zip", source_file_available: false })).toBe("review-pack.md");
  });

  it("falls back to the skill name when the source has no usable stem", () => {
    expect(skillFileName({ name: "Review", source_type: "zip", source_filename: ".zip", source_file_available: false })).toBe("Review.md");
  });

  it("flags only older ZIP skills as instructions-only", () => {
    expect(onlyInstructionsKept({ source_type: "zip", source_file_available: false })).toBe(true);
    expect(onlyInstructionsKept({ source_type: "zip", source_file_available: true })).toBe(false);
    expect(onlyInstructionsKept({ source_type: "markdown", source_file_available: false })).toBe(false);
  });
});
