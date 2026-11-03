#!/usr/bin/env python3
"""Exercise bootstrap branching with temporary commands; never install a service."""
import json
import hashlib
import io
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import tarfile


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
    sys.exit(int(os.environ.get('TEST_EXISTING','1')) if args[0]=='cat' else int(os.environ.get('TEST_SYSTEMD_EXIT','0')))
elif name=='getconf':
    print(os.environ.get('TEST_LIBC','glibc 2.36'))
elif name=='ldconfig':
    print(os.environ.get('TEST_STDLIB','libstdc++.so.6'))
elif name=='node':
    print(os.environ.get('TEST_NODE_MAJOR','22'))
elif name=='node-private':
    print('22')
elif name=='npm':
    sys.exit(int(os.environ.get('TEST_NPM_EXIT','0')))
elif name=='go':
    print('go version go'+os.environ.get('TEST_GO_VERSION','1.27.0')+' linux/amd64' if args==['version'] else 'fixture')
elif name=='go-private':
    print('go version go1.27.0 linux/amd64' if args==['version'] else 'fixture')
elif name=='git':
    shutil.copytree(os.environ['BOOTSTRAP_TEST_SOURCE'],args[-1])
elif name in ('apt-get','dnf','yum','pacman','zypper'):
    if os.environ.get('TEST_PACKAGE_EXIT'):
        sys.exit(int(os.environ['TEST_PACKAGE_EXIT']))
    for command in ['git','curl']:
        target=Path(os.environ['BOOTSTRAP_TEST_BIN'])/command
        if not target.exists() and os.environ.get('TEST_INSTALL_REPAIR','1')=='1':
            target.symlink_to(Path(os.environ['BOOTSTRAP_TEST_BIN'])/'stub')
elif name=='curl':
    if os.environ.get('TEST_CURL_EXIT'):
        sys.exit(int(os.environ['TEST_CURL_EXIT']))
    filename=args[-1].rsplit('/',1)[-1]
    content=(Path(os.environ['BOOTSTRAP_TEST_DOWNLOADS'])/filename).read_bytes()
    if filename.endswith('.tar.gz') and os.environ.get('TEST_CORRUPT'):
        content+=b'corrupted'
    if filename.endswith('.sha256') and os.environ.get('TEST_BAD_HASH'):
        content=b'bad-hash'
    if filename=='SHASUMS256.txt' and os.environ.get('TEST_BAD_MANIFEST'):
        content=b'bad-hash  ../../untrusted.tar.gz'
    Path(args[args.index('--output')+1]).write_bytes(content)
''', encoding='utf-8')
        stub.chmod(0o755)
        for name in ['git','curl','go','node','npm','uname','id','systemctl','getconf','ldconfig',
                     'apt-get','dnf','yum','pacman','zypper','go-private','node-private','npm-private']:
            (bindir/name).symlink_to(stub)
        # Hermetic PATH: never fall back to real package managers or toolchains.
        for name in ('bash sh env python3 awk sed sort cat chmod mktemp mkdir rm tar gzip '
                     'sha256sum cp mv install useradd groupadd getent find').split():
            target=shutil.which(name)
            assert target, 'test image needs '+name
            (bindir/name).symlink_to(target)
        downloads=work/'downloads'
        downloads.mkdir()
        def archive(filename,entries):
            path=downloads/filename
            with tarfile.open(path,'w:gz') as out:
                for name,content in entries.items():
                    data=content.encode()
                    item=tarfile.TarInfo(name)
                    item.mode=0o755
                    item.size=len(data)
                    out.addfile(item,io.BytesIO(data))
            return hashlib.sha256(path.read_bytes()).hexdigest()
        for arch in ['amd64','arm64']:
            filename=f'go1.27.0.linux-{arch}.tar.gz'
            checksum=archive(filename,{'go/bin/go':'#!/bin/sh\nexec go-private "$@"\n'})
            (downloads/(filename+'.sha256')).write_text(checksum+'\n')
        manifest=[]
        for arch in ['x64','arm64']:
            root=f'node-v22.99.0-linux-{arch}'
            filename=root+'.tar.gz'
            checksum=archive(filename,{
                root+'/bin/node':'#!/bin/sh\nexec node-private "$@"\n',
                root+'/bin/npm':'#!/bin/sh\nexec npm-private "$@"\n'})
            manifest.append(checksum+'  '+filename+'\n')
        (downloads/'SHASUMS256.txt').write_text(''.join(manifest))
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
            ('existing-unit', [], {'TEST_EXISTING':'0'}, 1, None, None),
            ('musl', [], {'TEST_LIBC':''}, 1, None, None),
            ('old-glibc', [], {'TEST_LIBC':'glibc 2.17'}, 1, None, None),
            ('old-go', [], {'TEST_GO_VERSION':'1.19.8'}, 0, 'full', 'amd64'),
            ('newer-go', [], {'TEST_GO_VERSION':'1.28.1'}, 0, 'full', 'amd64'),
            ('old-node', [], {'TEST_NODE_MAJOR':'16'}, 0, 'full', 'amd64'),
            ('node19', [], {'TEST_NODE_MAJOR':'19'}, 0, 'full', 'amd64'),
            ('node18', [], {'TEST_NODE_MAJOR':'18'}, 0, 'full', 'amd64'),
            ('npm-broken', [], {'TEST_NPM_EXIT':'1'}, 0, 'full', 'amd64'),
            ('missing-go', [], {'TEST_HIDE':'go'}, 0, 'full', 'amd64'),
            ('missing-node', [], {'TEST_HIDE':'node npm'}, 0, 'full', 'amd64'),
            ('arm64-toolchains', ['--lite'], {'TEST_ARCH':'aarch64','TEST_HIDE':'go node npm'}, 0, 'lite', 'arm64'),
            ('go-download-fails', [], {'TEST_HIDE':'go','TEST_CURL_EXIT':'22'}, 22, None, None),
            ('go-hash-mismatch', [], {'TEST_HIDE':'go','TEST_CORRUPT':'1'}, 1, None, None),
            ('go-hash-invalid', [], {'TEST_HIDE':'go','TEST_BAD_HASH':'1'}, 1, None, None),
            ('node-hash-mismatch', [], {'TEST_HIDE':'node','TEST_CORRUPT':'1'}, 1, None, None),
            ('node-manifest-invalid', [], {'TEST_HIDE':'node','TEST_BAD_MANIFEST':'1'}, 1, None, None),
            ('packages-fail', [], {'TEST_HIDE':'git','TEST_PACKAGE_EXIT':'42'}, 42, None, None),
            ('packages-incomplete', [], {'TEST_HIDE':'git','TEST_INSTALL_REPAIR':'0'}, 1, None, None),
            ('unsupported-distro-missing-deps', [], {'TEST_HIDE':'git','TEST_DISTRO':'unknown'}, 1, None, None),
            ('missing-stdlib', [], {'TEST_STDLIB':'','TEST_MANAGER':'apt-get'}, 0, 'full', 'amd64'),
            ('missing-curl', [], {'TEST_HIDE':'curl','TEST_MANAGER':'apt-get'}, 0, 'full', 'amd64'),
            ('unknown-distro-prepared', [], {'TEST_DISTRO':'unknown'}, 0, 'full', 'amd64'),
        ]
        for distro,manager in [('debian','apt-get'),('ubuntu','apt-get'),('fedora','dnf'),
                               ('rocky','dnf'),('almalinux','dnf'),('rhel','yum'),
                               ('arch','pacman'),('opensuse-leap','zypper')]:
            cases.append((distro+'-packages',['--lite'],
                          {'TEST_HIDE':'git'+(' dnf' if manager=='yum' else ''),
                           'TEST_DISTRO':distro,'TEST_MANAGER':manager},0,'lite','amd64'))
        cases.append(('id-like',[],{'TEST_HIDE':'git','TEST_DISTRO':'derivative',
                                   'TEST_ID_LIKE':'ubuntu debian','TEST_MANAGER':'apt-get'},0,'full','amd64'))
        for name,args,extra,want,variant,arch in cases:
            log.write_text('')
            script_log.write_text('')
            release=work/'os-release'
            release.write_text('ID='+extra.get('TEST_DISTRO','debian')+'\nID_LIKE="'+
                               extra.get('TEST_ID_LIKE','')+'"\nPRETTY_NAME="test fixture"\nVERSION="12 (bookworm)"\n')
            runner=work/'bootstrap.sh'
            runner.write_text(source.read_text().replace('/etc/os-release',str(release)))
            hidden=extra.get('TEST_HIDE','').split()
            for command in hidden:
                (bindir/command).unlink()
            env={**os.environ,'PATH':str(bindir),'TMPDIR':str(work),
                 'DUORELAY_VARIANT':'full','BOOTSTRAP_TEST_LOG':str(log),
                 'BOOTSTRAP_TEST_SOURCE':str(fixture),
                 'BOOTSTRAP_TEST_BIN':str(bindir),
                 'BOOTSTRAP_TEST_DOWNLOADS':str(downloads),
                 'BOOTSTRAP_TEST_SCRIPT_LOG':str(script_log),**extra}
            result=subprocess.run(['bash',str(runner),*args],env=env,text=True,capture_output=True)
            for command in hidden:
                if not (bindir/command).exists():
                    (bindir/command).symlink_to(stub)
            assert result.returncode==want,(name,result.stdout,result.stderr)
            calls=[json.loads(line) for line in log.read_text().splitlines()]
            scripts=script_log.read_text()
            if variant:
                if variant=='lite':
                    assert any(c in calls for c in [
                        [go,'run','./deploy/docker/download-kernels.go','-arch',arch,'-variant','lite','-out','kernels']
                        for go in ['go','go-private']]),(name,calls)
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
            if 'TEST_MANAGER' in extra:
                installs=[c for c in calls if c[0] in ['apt-get','dnf','yum','pacman','zypper']]
                assert installs and all(c[0]==extra['TEST_MANAGER'] for c in installs),(name,calls)
                if name=='missing-curl':
                    assert any('curl' in c for c in installs),'must install missing curl'
                else:
                    assert all('curl' not in c for c in installs),'avoid curl-minimal conflicts'
                assert all('-Sy' not in c for c in installs),'avoid partial Arch upgrades'
            fetched=[c[-1] for c in calls if c[0]=='curl']
            if name in ['old-go','missing-go','arm64-toolchains']:
                assert any('dl.google.com/go/go1.27.0' in u for u in fetched)
                assert any(c[0]=='go-private' for c in calls)
            if name in ['old-node','node19','npm-broken','missing-node','arm64-toolchains']:
                assert any('nodejs.org/dist/v22.99.0/' in u for u in fetched)
                assert any(c[0]=='node-private' for c in calls)
            if name in ['default-full','newer-go','node18','existing-unit']:
                assert not fetched,(name,calls)
            if name=='go-download-fails':
                assert '准备 Go 工具链' in result.stderr
            if name in ['go-hash-mismatch','node-hash-mismatch']:
                assert 'SHA-256 校验失败' in result.stderr
                assert not any(c[0] in ['go-private','node-private'] for c in calls)
            if name=='go-hash-invalid':
                assert '摘要格式无效' in result.stderr
            if name=='node-manifest-invalid':
                assert '没有唯一的匹配归档' in result.stderr
            assert not list(work.glob('duorelay-bootstrap.*')),'temporary directory leaked'
            print('PASS',name,flush=True)
        print(f'{len(cases)} isolated bootstrap cases passed.')


if __name__=='__main__':
    main()
