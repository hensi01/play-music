-- Keep this migration repeatable if version recording fails after DROP TABLE.
DO $$ DECLARE has_rows boolean; BEGIN
    IF to_regclass('karaokes') IS NOT NULL THEN
        EXECUTE 'SELECT EXISTS (SELECT 1 FROM karaokes)' INTO has_rows;
        IF has_rows THEN
            RAISE EXCEPTION 'karaoke records remain: run cmd/remove-karaoke while the app is stopped before migration 0010';
        END IF;
    END IF;
END $$;
DELETE FROM artworks WHERE entity_type IN ('karaoke', 'karaoke_thumb');
DROP TABLE IF EXISTS category_karaokes;
DROP TABLE IF EXISTS karaokes;
