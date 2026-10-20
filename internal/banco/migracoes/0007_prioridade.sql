-- Triagem: prioridade escolhida à mão; sem ela, o requisito ainda não foi triado.
ALTER TABLE requisitos ADD COLUMN prioridade TEXT CHECK (prioridade IN ('alta', 'media', 'baixa'));
