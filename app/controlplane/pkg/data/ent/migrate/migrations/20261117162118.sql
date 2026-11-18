-- Since protoyaml v0.8.0 (Chainloop v1.135.3), a message field set to null in YAML stays unset.
-- A stored YAML contract with a null top-level spec (`spec:`, `spec: null` or `spec: ~`) then fails
-- validation each time the control plane reads it. Before v1.135.3, the same text gave an empty spec.
-- This rewrites the value to `spec: {}`, which keeps that meaning. Rows that are not UTF-8 stay as they are.
DO $$
DECLARE
    r record;
    doc text;
    fixed text;
BEGIN
    FOR r IN
        SELECT "id", "raw_body" FROM "workflow_contract_versions"
        WHERE "raw_body_format" = 'yaml' AND position('spec:'::bytea IN "raw_body") > 0
    LOOP
        BEGIN
            doc := convert_from(r.raw_body, 'UTF8');
        EXCEPTION WHEN character_not_in_repertoire OR untranslatable_character THEN
            CONTINUE;
        END;

        -- E'' strings keep the backslashes when standard_conforming_strings is off.
        -- `spec: null` or `spec: ~`, with an optional comment
        fixed := regexp_replace(doc,
            E'^spec:[ \\t]+(?:null|Null|NULL|~)([ \\t]+#[^\\r\\n]*)?[ \\t]*(\\r?)$',
            E'spec: {}\\1\\2', 'n');
        -- `spec:` with no indented content line after it
        fixed := regexp_replace(fixed,
            E'^spec:([ \\t]+#[^\\r\\n]*)?[ \\t]*(\\r?)$(?!\\n(?:[ \\t]*(?:#[^\\r\\n]*)?\\r?\\n)*[ \\t]+[^ \\t\\r\\n#])',
            E'spec: {}\\1\\2', 'n');

        IF fixed <> doc THEN
            UPDATE "workflow_contract_versions" SET "raw_body" = convert_to(fixed, 'UTF8') WHERE "id" = r.id;
        END IF;
    END LOOP;
END $$;
