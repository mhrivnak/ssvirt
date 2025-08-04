-- Drop tables in reverse order to handle foreign key constraints
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS vms;
DROP TABLE IF EXISTS vapps;
DROP TABLE IF EXISTS vapp_templates;
DROP TABLE IF EXISTS catalogs;
DROP TABLE IF EXISTS vdcs;
DROP TABLE IF EXISTS organizations;