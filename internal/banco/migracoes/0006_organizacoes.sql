-- Cada organização tem o seu zero: o requisito do qual todos os outros dela derivam. O zero geral
-- (o único sem pais) continua sendo o da visão sem organização.
CREATE TABLE organizacoes (
    nome    TEXT PRIMARY KEY CHECK (length(trim(nome)) > 0),
    zero_id INTEGER REFERENCES requisitos (id)
);
