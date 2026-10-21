-- Pontos de importância na ligação pai-filho: só se comparam entre irmãos. Sem pontos, a ligação
-- ainda não foi triada (conta como 1). A prioridade dos requisitos fica só como histórico.
ALTER TABLE derivacoes ADD COLUMN pontos INTEGER CHECK (pontos IS NULL OR pontos >= 1);
