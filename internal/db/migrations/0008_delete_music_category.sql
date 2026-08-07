-- Remove definitivamente a categoria "music" (atribuicoes de musicas e
-- liberacoes de usuarios sao removidas via ON DELETE CASCADE; as musicas
-- permanecem no acervo, apenas ficam sem categoria).
DELETE FROM categories WHERE lower(name) = 'music';
