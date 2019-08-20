CREATE OR REPLACE FUNCTION drop_gphdfs() RETURNS VOID AS $$
DECLARE
  rolerow RECORD;
BEGIN
  RAISE NOTICE 'Dropping gphdfs users...';
  FOR rolerow IN SELECT * FROM pg_catalog.pg_roles LOOP
    EXECUTE 'alter role '
      || quote_ident(rolerow.rolname) || ' '
      || 'NOCREATEEXTTABLE(protocol=''gphdfs'',type=''readable'')';
    EXECUTE 'alter role '
      || quote_ident(rolerow.rolname) || ' '
      || 'NOCREATEEXTTABLE(protocol=''gphdfs'',type=''writable'')';
    RAISE NOTICE 'dropping gphdfs from role % ...', quote_ident(rolerow.rolname);
  END LOOP;
END;
$$ LANGUAGE plpgsql;


SELECT drop_gphdfs();

DROP FUNCTION drop_gphdfs();