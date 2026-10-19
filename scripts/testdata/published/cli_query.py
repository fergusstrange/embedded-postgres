import json,os,pathlib,subprocess,sys
cache=pathlib.Path(sys.argv[1]);record=pathlib.Path(sys.argv[2])
psql=[x for x in cache.rglob('psql*') if x.name in ('psql','psql.exe') and 'postgresql-18.6.1-'in str(x)]
assert len(psql)==1
result=subprocess.check_output([str(psql[0]),os.environ['DATABASE_URL'],'-Atc','SELECT 42; SHOW server_version_num;'],text=True)
assert result.strip()=='42\n180006',result
record.write_text(json.dumps({'port':int(os.environ['PGPORT']),'sql':42,'server_version_num':180006})+'\n')
print('Published CLI: default PostgreSQL 18.6.1 and SQL verified')
