import { describe, expect, it } from "vitest";
import { GREENNODE_MODELS, modelLogo, providerKindLabel } from "./provider-catalog";

describe("provider catalog", () => {
  it("labels every supported provider kind", () => {
    expect(providerKindLabel("gemini")).toBe("Google Gemini");
    expect(providerKindLabel("greennode")).toBe("GreenNode");
    expect(providerKindLabel("openai_compatible")).toBe("Company AI server");
  });

  it("maps the curated GreenNode models to their brand assets", () => {
    expect(GREENNODE_MODELS.map((model) => model.id)).toEqual(["z-ai/glm-5.2-hackathon", "qwen/qwen3.6-flash"]);
    expect(modelLogo(GREENNODE_MODELS[0]!.id)).toMatchObject({ src: "/model-logos/zai.svg", monochrome: true });
    expect(modelLogo(GREENNODE_MODELS[1]!.id)).toMatchObject({ src: "/model-logos/qwen.svg", monochrome: false });
    expect(modelLogo("other/model")).toBeNull();
  });
});
