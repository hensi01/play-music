-- Adiciona o link de checkout da categoria (loja externa).
ALTER TABLE categories ADD COLUMN IF NOT EXISTS checkout_url TEXT NOT NULL DEFAULT '';
