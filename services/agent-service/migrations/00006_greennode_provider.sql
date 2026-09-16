-- +goose Up
ALTER TABLE llm_providers DROP CONSTRAINT llm_providers_kind_check;
ALTER TABLE llm_providers DROP CONSTRAINT llm_providers_check;
ALTER TABLE llm_providers
    ADD CONSTRAINT llm_providers_kind_check
    CHECK (kind IN ('gemini', 'greennode', 'openai_compatible'));
ALTER TABLE llm_providers
    ADD CONSTRAINT llm_providers_check
    CHECK (kind <> 'openai_compatible' OR (base_url IS NOT NULL AND length(base_url) > 0));

-- +goose Down
ALTER TABLE llm_providers DROP CONSTRAINT llm_providers_kind_check;
ALTER TABLE llm_providers DROP CONSTRAINT llm_providers_check;
ALTER TABLE llm_providers
    ADD CONSTRAINT llm_providers_kind_check
    CHECK (kind IN ('gemini', 'openai_compatible'));
ALTER TABLE llm_providers
    ADD CONSTRAINT llm_providers_check
    CHECK (kind <> 'openai_compatible' OR (base_url IS NOT NULL AND length(base_url) > 0));
