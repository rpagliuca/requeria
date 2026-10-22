-- rex voz sem o intermediário do atalho da Siri: sai `voz_proposto`, o ponteiro entregue à Siri e
-- ainda não confirmado. Só o conector do app do Claude marca como ouvido, direto em `voz_lido_ate`.
ALTER TABLE sessoes DROP COLUMN voz_proposto;
