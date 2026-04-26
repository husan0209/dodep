ALTER TABLE affiliate_links
    ADD COLUMN IF NOT EXISTS referral_url TEXT;

ALTER TABLE affiliate_payout_methods
    ADD COLUMN IF NOT EXISTS display_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS details_masked TEXT NOT NULL DEFAULT '';

UPDATE affiliate_links
SET referral_url = '/r/' || referral_code ||
    CASE
        WHEN campaign_name IS NOT NULL AND campaign_name <> '' THEN '/' || campaign_name
        ELSE ''
    END
WHERE referral_url IS NULL;

UPDATE affiliate_payout_methods
SET display_name = COALESCE(NULLIF(display_name, ''), method_type::text),
    details_masked = COALESCE(NULLIF(details_masked, ''), details_encrypted)
WHERE display_name = '' OR details_masked = '';
