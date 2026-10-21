-- Pontos de importância na ligação pai-filho: só se comparam entre irmãos. Sem pontos, a ligação
-- ainda não foi triada (conta como 1). A prioridade dos requisitos fica só como histórico.
ALTER TABLE derivacoes ADD COLUMN pontos INTEGER CHECK (pontos IS NULL OR pontos >= 1);

-- A prioridade já gravada vira pontos em todas as ligações do requisito com os pais: alta 9, média 3,
-- baixa 1. Sem prioridade, a ligação fica sem pontos.
UPDATE derivacoes SET pontos = (
    SELECT CASE r.prioridade WHEN 'alta' THEN 9 WHEN 'media' THEN 3 WHEN 'baixa' THEN 1 END
    FROM requisitos r WHERE r.id = derivacoes.requisito_id);
