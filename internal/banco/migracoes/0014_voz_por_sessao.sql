-- rex voz com intermediário permanente: sai a sessão associada (o intermediário escolhe a sessão a
-- cada pedido). `voz_proposto` é, em cada sessão, o ponteiro entregue ao intermediário e ainda não
-- confirmado pelo atalho.
DROP TABLE voz;
ALTER TABLE sessoes ADD COLUMN voz_proposto TEXT;
