CREATE TABLE IF NOT EXISTS products (
  id BIGSERIAL PRIMARY KEY,
  code VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_products_deleted_at ON products(deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS ux_products_code_active ON products(code) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS product_lines (
  product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  line_id BIGINT NOT NULL REFERENCES lines(id) ON DELETE CASCADE,
  PRIMARY KEY (product_id, line_id)
);
CREATE INDEX IF NOT EXISTS idx_product_lines_line ON product_lines(line_id);
