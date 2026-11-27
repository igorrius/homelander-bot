-- MCP: a user flag that lets an admin connect an AI agent (spec 6.2).
ALTER TABLE users ADD COLUMN is_mcp INTEGER NOT NULL DEFAULT 0 CHECK (is_mcp IN (0,1));
