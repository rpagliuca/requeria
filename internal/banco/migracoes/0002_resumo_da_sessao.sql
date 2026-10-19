-- Cada sessão guarda o próprio resumo; a tarefa continua com uma cópia do último.
ALTER TABLE sessoes ADD COLUMN resumo TEXT;

-- Preenche as sessões já encerradas com o resumo atual da tarefa, quando a sessão é a última
-- encerrada dela. As anteriores não têm como ser recuperadas e ficam sem resumo.
DROP TRIGGER sessoes_encerradas_imutaveis;

UPDATE sessoes SET resumo = (
    SELECT t.resumo FROM sessao_tarefa st JOIN tarefas t ON t.id = st.tarefa_id
    WHERE st.sessao_id = sessoes.id AND t.resumo <> ''
    LIMIT 1)
WHERE status = 'encerrada' AND id IN (
    SELECT (SELECT s2.id FROM sessao_tarefa st2 JOIN sessoes s2 ON s2.id = st2.sessao_id
            WHERE st2.tarefa_id = st.tarefa_id AND s2.status = 'encerrada'
            ORDER BY s2.encerrada_em DESC, s2.id DESC LIMIT 1)
    FROM sessao_tarefa st);

CREATE TRIGGER sessoes_encerradas_imutaveis
BEFORE UPDATE ON sessoes
WHEN OLD.status <> 'ativa'
BEGIN
    SELECT RAISE(ABORT, 'uma sessão encerrada não pode ser alterada');
END;
