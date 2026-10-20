-- Execuções: verificações de conformidade de um requisito, com prazo de validade opcional.
CREATE TABLE execucoes (
    id            INTEGER PRIMARY KEY,
    requisito_id  INTEGER NOT NULL REFERENCES requisitos (id),
    evidencia     TEXT NOT NULL CHECK (length(trim(evidencia)) > 0),
    realizada_em  TEXT NOT NULL,
    valida_ate    TEXT,
    autor         TEXT NOT NULL,
    registrada_em TEXT NOT NULL
);
CREATE INDEX execucoes_por_requisito ON execucoes (requisito_id, realizada_em);
