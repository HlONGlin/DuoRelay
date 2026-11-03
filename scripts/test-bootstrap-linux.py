#!/usr/bin/env python3
"""Exercise bootstrap branching with temporary commands; never install a service."""
import json
import os
from pathlib import Path
import subprocess
import tempfile


def main():
    if os.name != 'posix':
        raise SystemExit('Run this test on Linux (a container is sufficient).')
    source = Path(__file__).resolve().with_name('bootstrap-linux.sh')
    with tempfile.TemporaryDirectory(prefix='duorelay-bootstrap-test-') as temporary:
        work = Path(temporary)
        bindir = work / 'bin'
        bindir.mkdir()
        log = work / 'calls.jsonl'
        stub = bindir / 'stub'
        stub.write_text('''#!/usr/bin/env python3
import json,os,sys,shutil
from pathlib import Path
name=Path(sys.argv[0]).name
args=sys.argv[1:]
with open(os.environ['BOOTSTRAP_TEST_LOG'],'a') as out:
    out.write(json.dumps([name,*args])+'\\n')
if name=='uname':
    print('Linux' if args==['-s'] else os.environ.get('TEST_ARCH','x86_64'))
elif name=='id':
    print(os.environ.get('TEST_UID','0'))
elif name=='systemctl':
    sys.exit(int(os.environ.get('TEST_SYSTEMD_EXIT','0')))
elif name=='node':
    print(os.environ.get('TEST_NODE_MAJOR','22'))
elif name=='go':
    print('go version go1.27.0 linux/amd64' if args==['version'] else 'fixture')
elif name=='git':
    shutil.copytree(os.environ['BOOTSTRAP_TEST_SOURCE'],args[-1])
''', encoding='utf-8')
        stub.chmod(0o755)
        for name in ['git','curl','go','node','npm','uname','id','systemctl']:
            (bindir/name).symlink_to(stub)
        fixture=work/'source'
        (fixture/'scripts').mkdir(parents=True)
        (fixture/'web').mkdir()
        (fixture/'go.mod').write_text('module fixture\n\ngo 1.27.0\n')
        for script in ['download-kernels.sh','package-linux.sh','install-linux.sh']:
            text='#!/bin/bash\nset -eu\nprintf \'%s\\n\' "'+script+' $*" >> "$BOOTSTRAP_TEST_SCRIPT_LOG"\n'
            if script=='package-linux.sh':
                text+='if [[ "${3:-}" == --lite ]]; then mkdir -p "dist/staging-linux/duorelay-lite-$1-$2"; else mkdir -p "dist/duorelay-linux-$2-$1"; fi\n'
            if script=='install-linux.sh':
                text+='test "$1" = --package\ntest -d "$2"\n'
            (fixture/'scripts'/script).write_text(text)
        script_log=work/'scripts.log'
        cases = [
            ('default-full', [], {}, 0, 'full', 'amd64'),
            ('explicit-full', ['--full'], {}, 0, 'full', 'amd64'),
            ('lite-amd64', ['--lite'], {}, 0, 'lite', 'amd64'),
            ('lite-arm64', ['--lite'], {'TEST_ARCH':'aarch64'}, 0, 'lite', 'arm64'),
            ('full-arm64', ['--full'], {'TEST_ARCH':'aarch64'}, 0, 'full', 'arm64'),
            ('env-lite', [], {'DUORELAY_VARIANT':'lite'}, 0, 'lite', 'amd64'),
            ('argument-overrides-env', ['--full'], {'DUORELAY_VARIANT':'lite'}, 0, 'full', 'amd64'),
            ('help', ['--help'], {}, 0, None, None),
            ('invalid-option', ['--unknown'], {}, 1, None, None),
            ('invalid-variant', [], {'DUORELAY_VARIANT':'bad'}, 1, None, None),
            ('unsupported-arch', [], {'TEST_ARCH':'armv7l'}, 1, None, None),
            ('not-root', [], {'TEST_UID':'1000'}, 1, None, None),
            ('no-systemd', [], {'TEST_SYSTEMD_EXIT':'1'}, 1, None, None),
            ('old-node', [], {'TEST_NODE_MAJOR':'16'}, 1, None, None),
        ]
        for name,args,extra,want,variant,arch in cases:
            log.write_text('')
            script_log.write_text('')
            env={**os.environ,'PATH':str(bindir)+os.pathsep+os.environ['PATH'],
                 'DUORELAY_VARIANT':'full','BOOTSTRAP_TEST_LOG':str(log),
                 'BOOTSTRAP_TEST_SOURCE':str(fixture),
                 'BOOTSTRAP_TEST_SCRIPT_LOG':str(script_log),**extra}
            result=subprocess.run(['bash',str(source),*args],env=env,text=True,capture_output=True)
            assert result.returncode==want,(name,result.stdout,result.stderr)
            calls=[json.loads(line) for line in log.read_text().splitlines()]
            scripts=script_log.read_text()
            if variant:
                if variant=='lite':
                    assert ['go','run','./deploy/docker/download-kernels.go','-arch',arch,'-variant','lite','-out','kernels'] in calls,(name,calls)
                    assert 'download-kernels.sh' not in scripts
                    assert f'package-linux.sh bootstrap {arch} --lite' in scripts
                    assert f'/dist/staging-linux/duorelay-lite-bootstrap-{arch}' in scripts
                else:
                    assert 'download-kernels.sh' in scripts
                    assert f'package-linux.sh bootstrap {arch}\n' in scripts
                    assert f'/dist/duorelay-linux-{arch}-bootstrap' in scripts
                assert 'install-linux.sh --package ' in scripts
            else:
                assert 'install-linux.sh' not in scripts,(name,scripts)
            print('PASS',name,flush=True)


if __name__=='__main__':
    main()
