// One pipeline for both halves of the repo: the Go API and the Next.js frontend.
// They deploy to the same host, so a single Jenkinsfile keeps "the API and the UI
// that talks to it" moving together instead of drifting between two jobs.
pipeline {
    agent any

    options {
        timeout(time: 30, unit: 'MINUTES')
        disableConcurrentBuilds()
        buildDiscarder(logRotator(numToKeepStr: '10'))
    }

    parameters {
        choice(name: 'TARGET', choices: ['both', 'backend', 'frontend'], description: 'Build/deploy เฉพาะส่วนไหน (both = ค่า default สำหรับ auto-trigger)')
    }

    environment {
        APP_NAME = 'filemepls-api'
        // Belt and braces: the binary also reads GIN_MODE from its .env, which is
        // what applies when it is started by hand.
        GIN_MODE = 'release'
        CI       = 'true'
        PATH     = "/usr/local/go/bin:${PATH}"
    }

    stages {
        stage('Checkout') {
            steps {
                checkout scm
                script {
                    def raw = env.BRANCH_NAME ?: env.GIT_BRANCH ?: ''
                    env.DEPLOY_BRANCH = raw.replaceFirst(/^origin\//, '')
                    echo "Resolved branch: ${env.DEPLOY_BRANCH}"

                    // Only main deploys. Other branches still build/vet/test — a PR
                    // branch failing the pipeline is useful; a PR branch overwriting
                    // the live release directory is not.
                    if (env.DEPLOY_BRANCH == 'main') {
                        env.DEPLOY = 'yes'

                        env.BE_DEPLOY_DIR = '/home/ubuntu/deploy/filemepls-api'
                        env.BE_SERVICE    = 'filemepls-api'
                        env.BE_PORT       = '8008'   // must match HTTP_ADDR in $BE_ENV_FILE
                        env.BE_ENV_FILE   = '/home/ubuntu/code/filemepls/backend/.env'

                        env.FE_DEPLOY_DIR = '/home/ubuntu/deploy/filemepls-web'
                        env.FE_SERVICE    = 'filemepls-web'
                        env.FE_PORT       = '3003'   // must match the proxy_pass in the nginx server block
                        env.FE_ENV_FILE   = '/home/ubuntu/code/filemepls/frontend/.env.local'
                    } else {
                        env.DEPLOY = 'no'
                        echo "Branch '${env.DEPLOY_BRANCH}' is not main - build/test only, no deploy"
                    }
                }
                sh 'git log -1 --pretty=format:"%h %an %ad %s"'
                sh 'go version'
                sh 'node --version && npm --version'
            }
        }

        stage('Backend') {
            when { expression { params.TARGET == 'both' || params.TARGET == 'backend' } }
            stages {
                stage('Build') {
                    steps {
                        // No separate migrate binary to ship: migrations are embedded and
                        // RunMigrations runs at startup (cmd/api/main.go), so the schema
                        // moves when the new binary boots. That is what the pg_dump in
                        // Deploy is guarding.
                        dir('backend') {
                            sh 'CGO_ENABLED=0 go build -o "$APP_NAME" ./cmd/api/'
                        }
                    }
                }

                stage('Vet') {
                    steps {
                        dir('backend') { sh 'go vet ./...' }
                    }
                }

                stage('Test') {
                    steps {
                        // Every suite here is a unit test against fakes - nothing reaches
                        // Postgres - so this stage needs no database and nothing self-skips.
                        // If a DB-backed suite is ever added, give it its own env var and a
                        // skip, rather than making this stage depend on a live database.
                        //
                        // No -race: it needs cgo and the binary above is built CGO_ENABLED=0.
                        dir('backend') { sh 'go test ./... -count=1' }
                    }
                }

                stage('Deploy Backend') {
                    when { expression { env.DEPLOY == 'yes' } }
                    steps {
                        echo "Deploying backend -> pm2 '${env.BE_SERVICE}' at '${env.BE_DEPLOY_DIR}' (port ${env.BE_PORT})"

                        // Single-quoted sh: the SHELL (not Groovy) expands $BE_* / $APP_NAME,
                        // which Jenkins exports from the environment above. pm2 runs as
                        // 'ubuntu' with -H so it uses /home/ubuntu/.pm2; the jenkins user
                        // cannot write under /home/ubuntu itself.
                        //
                        // Releases are directories under $BE_DEPLOY_DIR/releases and
                        // $BE_DEPLOY_DIR/current is a symlink to the live one, so a build
                        // that fails its health check can be put back.
                        dir('backend') {
                            sh '''
                                set -eu

                                REL_NAME="$(date +%Y%m%d%H%M%S)-$(echo "${GIT_COMMIT:-nogit}" | cut -c1-8)"
                                REL="$BE_DEPLOY_DIR/releases/$REL_NAME"
                                # readlink WITHOUT -f: -f prints the path even when it is not a
                                # symlink at all, which would record ".../current" as its own
                                # predecessor on the first deploy and make rollback a no-op.
                                PREV="$(sudo -H -u ubuntu readlink "$BE_DEPLOY_DIR/current" 2>/dev/null || true)"

                                echo "=== Staging release $REL_NAME ==="
                                sudo -H -u ubuntu mkdir -p "$REL/data" "$BE_DEPLOY_DIR/backups" "$BE_DEPLOY_DIR/storage"
                                sudo -H -u ubuntu cp -f "$APP_NAME" "$REL/$APP_NAME"
                                sudo -H -u ubuntu chmod +x "$REL/$APP_NAME"

                                # Reuse the hand-managed .env that already lives in ubuntu's own
                                # checkout. Secrets never pass through the jenkins user; this
                                # pipeline does not create or edit $BE_ENV_FILE itself.
                                sudo -H -u ubuntu test -f "$BE_ENV_FILE" || { echo "ERROR: $BE_ENV_FILE not found - create it by hand first"; exit 1; }
                                sudo -H -u ubuntu cp -f "$BE_ENV_FILE" "$REL/.env"
                                sudo -H -u ubuntu chmod 600 "$REL/.env"

                                # Uploaded files are state, not build output. They stay in
                                # $BE_DEPLOY_DIR/storage and each release links to them, so a new
                                # release never starts with an empty store. Only matters while
                                # STORAGE_ROOT is the default "./data/storage", which resolves
                                # against --cwd; an absolute STORAGE_ROOT makes it irrelevant.
                                sudo -H -u ubuntu ln -sfn "$BE_DEPLOY_DIR/storage" "$REL/data/storage"

                                echo "=== Backup before the new binary migrates ==="
                                # Has to happen before the swap and has to fail the build if it
                                # fails: restoring is the only real rollback for a schema change.
                                # The symlink swap below only covers the binary.
                                command -v pg_dump >/dev/null || { echo "ERROR: pg_dump not installed on this host"; exit 1; }
                                # The script arrives on stdin with a quoted heredoc and the paths
                                # come in as $1/$2, so nothing here is expanded by the traced outer
                                # shell - the DSN password never reaches the build log. And -Z
                                # replaces a `| gzip`, whose exit status would be gzip's: pg_dump
                                # could fail while the build sailed on with no backup at all.
                                sudo -H -u ubuntu sh -s "$REL" "$BE_DEPLOY_DIR/backups/pre-$REL_NAME.sql.gz" <<'BACKUP'
set -eu
cd "$1"
url=$(grep -m1 '^DATABASE_URL=' .env | cut -d= -f2- | sed 's/^"//; s/"$//')
[ -n "$url" ] || { echo "ERROR: DATABASE_URL is not in .env - refusing to deploy without a backup"; exit 1; }
pg_dump -Z 9 -f "$2" "$url"
BACKUP
                                sudo -H -u ubuntu sh -c "cd '$BE_DEPLOY_DIR/backups' && ls -1t | tail -n +11 | xargs -r rm -f"

                                echo "=== Switching current -> $REL_NAME ==="
                                # ln -sfn onto an existing symlink is not atomic; mv -T over it is.
                                sudo -H -u ubuntu ln -sfn "$REL" "$BE_DEPLOY_DIR/current.tmp"
                                sudo -H -u ubuntu mv -Tf "$BE_DEPLOY_DIR/current.tmp" "$BE_DEPLOY_DIR/current"

                                # reload, not delete+start: pm2 brings the new process up before
                                # retiring the old one. But reload keeps the exec path pm2 recorded
                                # at `pm2 start` time, so a process still pointing at an older
                                # layout would restart unchanged against a schema this build just
                                # migrated. Only reload when pm2 is already running out of current/.
                                if sudo -H -u ubuntu pm2 jlist 2>/dev/null | grep -q "pm_exec_path.:.$BE_DEPLOY_DIR/current/$APP_NAME"; then
                                    sudo -H -u ubuntu pm2 reload "$BE_SERVICE" --update-env
                                else
                                    sudo -H -u ubuntu pm2 delete "$BE_SERVICE" >/dev/null 2>&1 || true
                                    sudo -H -u ubuntu env GIN_MODE="$GIN_MODE" pm2 start "$BE_DEPLOY_DIR/current/$APP_NAME" \
                                        --name "$BE_SERVICE" \
                                        --interpreter none \
                                        --cwd "$BE_DEPLOY_DIR/current" \
                                        --max-memory-restart 800M \
                                        --time
                                fi

                                echo "=== Health check (http://localhost:$BE_PORT/readyz) ==="
                                # /readyz, not /healthz: healthz answers 200 as soon as the HTTP
                                # server is listening, so it would pass a build that cannot reach
                                # Postgres at all - the failure this gate exists to catch.
                                ok=0
                                for i in $(seq 1 20); do
                                    if curl -fsS "http://localhost:$BE_PORT/readyz" >/dev/null 2>&1; then
                                        ok=1; break
                                    fi
                                    sleep 2
                                done

                                if [ "$ok" != "1" ]; then
                                    echo "ERROR: $BE_SERVICE did not become ready on port $BE_PORT"
                                    sudo -H -u ubuntu pm2 logs "$BE_SERVICE" --lines 80 --nostream || true
                                    if [ -n "$PREV" ] && [ -d "$PREV" ]; then
                                        echo "=== Rolling back to $(basename "$PREV") ==="
                                        sudo -H -u ubuntu ln -sfn "$PREV" "$BE_DEPLOY_DIR/current.tmp"
                                        sudo -H -u ubuntu mv -Tf "$BE_DEPLOY_DIR/current.tmp" "$BE_DEPLOY_DIR/current"
                                        sudo -H -u ubuntu pm2 reload "$BE_SERVICE" --update-env || true
                                        echo "NOTE: the binary is back on the previous release, but any migration"
                                        echo "      the new one applied at startup is still in place. If the new"
                                        echo "      schema is what broke it, restore"
                                        echo "      $BE_DEPLOY_DIR/backups/pre-$REL_NAME.sql.gz by hand."
                                    else
                                        echo "NOTE: no previous release to roll back to."
                                    fi
                                    exit 1
                                fi

                                sudo -H -u ubuntu pm2 save
                                # Keep the last five releases: enough to step back more than once,
                                # few enough not to fill the disk with binaries.
                                sudo -H -u ubuntu sh -c "cd '$BE_DEPLOY_DIR/releases' && ls -1t | tail -n +6 | xargs -r rm -rf"
                                echo "=== $BE_SERVICE ready on $REL_NAME ==="
                            '''
                        }
                    }
                }
            }
        }

        stage('Frontend') {
            when { expression { params.TARGET == 'both' || params.TARGET == 'frontend' } }
            stages {
                stage('Install') {
                    steps {
                        // ci, not install: fails if package.json and the lockfile disagree,
                        // and installs exactly what the lockfile pins. package-lock.json is
                        // never deleted here - re-resolving every caret range each build is
                        // how two builds of the same commit end up different bundles.
                        dir('frontend') {
                            sh 'rm -rf node_modules .next'
                            sh 'npm ci --no-fund'
                        }
                    }
                }

                stage('Lint') {
                    steps {
                        dir('frontend') { sh 'npm run lint' }
                    }
                }

                stage('Build') {
                    steps {
                        // NEXT_PUBLIC_* are baked into the compiled JS at build time, so the
                        // env file has to be in place BEFORE next build - editing it on the
                        // server afterwards does nothing until the next build. Read as ubuntu
                        // because the file lives in ubuntu's checkout; see frontend/.env.example.
                        dir('frontend') {
                            sh '''
                                set -eu
                                sudo -H -u ubuntu test -f "$FE_ENV_FILE" || { echo "ERROR: $FE_ENV_FILE not found - create it by hand first (see frontend/.env.example)"; exit 1; }
                                sudo -H -u ubuntu cat "$FE_ENV_FILE" > .env.local
                            '''
                            sh 'npm run build'
                            // The `postbuild` script copies public/ and .next/static into
                            // .next/standalone - server.js does not do it itself. Without them
                            // the site renders but every asset 404s, which is exactly how prod
                            // shipped a page with no CSS. Assert it here rather than finding out
                            // from the browser console.
                            sh '''
                                set -eu
                                test -f .next/standalone/server.js       || { echo "standalone server.js missing - is output:'standalone' still set in next.config.ts?"; exit 1; }
                                test -d .next/standalone/.next/static    || { echo ".next/static not copied into standalone - check the postbuild script"; exit 1; }
                                test -d .next/standalone/public          || { echo "public/ not copied into standalone - check the postbuild script"; exit 1; }
                            '''
                        }
                    }
                }

                stage('Deploy Frontend') {
                    when { expression { env.DEPLOY == 'yes' } }
                    steps {
                        echo "Deploying frontend -> pm2 '${env.FE_SERVICE}' at '${env.FE_DEPLOY_DIR}' (port ${env.FE_PORT})"

                        // Same release/symlink layout as the backend. It matters more here:
                        // rsync --delete straight onto the live directory would pull the JS
                        // chunks out from under every browser mid-deploy.
                        //
                        // .next/standalone carries its own trimmed node_modules, so the whole
                        // deploy is one rsync of that directory - nothing else to install.
                        sh '''
                            set -eu

                            REL_NAME="$(date +%Y%m%d%H%M%S)-$(echo "${GIT_COMMIT:-nogit}" | cut -c1-8)"
                            REL="$FE_DEPLOY_DIR/releases/$REL_NAME"
                            PREV="$(sudo -H -u ubuntu readlink "$FE_DEPLOY_DIR/current" 2>/dev/null || true)"

                            echo "=== Staging release $REL_NAME ==="
                            sudo -H -u ubuntu mkdir -p "$REL"
                            sudo -H -u ubuntu rsync -rlptD --delete "$WORKSPACE/frontend/.next/standalone/" "$REL/"

                            echo "=== Switching current -> $REL_NAME ==="
                            sudo -H -u ubuntu ln -sfn "$REL" "$FE_DEPLOY_DIR/current.tmp"
                            sudo -H -u ubuntu mv -Tf "$FE_DEPLOY_DIR/current.tmp" "$FE_DEPLOY_DIR/current"

                            # HOSTNAME=127.0.0.1: nginx is on this host and terminates TLS, so
                            # the Node server has no reason to be reachable from the network.
                            if sudo -H -u ubuntu pm2 jlist 2>/dev/null | grep -q "pm_exec_path.:.$FE_DEPLOY_DIR/current/server.js"; then
                                sudo -H -u ubuntu pm2 reload "$FE_SERVICE" --update-env
                            else
                                sudo -H -u ubuntu pm2 delete "$FE_SERVICE" >/dev/null 2>&1 || true
                                sudo -H -u ubuntu env NODE_ENV=production PORT="$FE_PORT" HOSTNAME=127.0.0.1 pm2 start "$FE_DEPLOY_DIR/current/server.js" \
                                    --name "$FE_SERVICE" \
                                    --cwd "$FE_DEPLOY_DIR/current" \
                                    --max-memory-restart 800M \
                                    --time
                            fi

                            echo "=== Health check (http://127.0.0.1:$FE_PORT/th) ==="
                            # Fetches the page AND one stylesheet it references. A plain 200 on
                            # /th proves only that Node is up: the missing-static outage served
                            # a perfectly good 200 with every asset behind it 404ing.
                            ok=0
                            for i in $(seq 1 20); do
                                html="$(curl -fsS "http://127.0.0.1:$FE_PORT/th" 2>/dev/null || true)"
                                css="$(printf '%s' "$html" | grep -o '/_next/static/[^"]*\\.css' | head -1 || true)"
                                if [ -n "$css" ] && curl -fsS -o /dev/null "http://127.0.0.1:$FE_PORT$css" 2>/dev/null; then
                                    ok=1; break
                                fi
                                sleep 2
                            done

                            if [ "$ok" != "1" ]; then
                                echo "ERROR: $FE_SERVICE did not serve /th plus its stylesheet on port $FE_PORT"
                                sudo -H -u ubuntu pm2 logs "$FE_SERVICE" --lines 80 --nostream || true
                                if [ -n "$PREV" ] && [ -d "$PREV" ]; then
                                    echo "=== Rolling back to $(basename "$PREV") ==="
                                    sudo -H -u ubuntu ln -sfn "$PREV" "$FE_DEPLOY_DIR/current.tmp"
                                    sudo -H -u ubuntu mv -Tf "$FE_DEPLOY_DIR/current.tmp" "$FE_DEPLOY_DIR/current"
                                    sudo -H -u ubuntu pm2 reload "$FE_SERVICE" --update-env || true
                                else
                                    echo "NOTE: no previous release to roll back to."
                                fi
                                exit 1
                            fi

                            sudo -H -u ubuntu pm2 save
                            sudo -H -u ubuntu sh -c "cd '$FE_DEPLOY_DIR/releases' && ls -1t | tail -n +6 | xargs -r rm -rf"
                            echo "=== $FE_SERVICE serving $REL_NAME ==="
                        '''
                    }
                }
            }
        }
    }

    post {
        success {
            sh 'sudo -H -u ubuntu pm2 list || true'
            echo "✅ TARGET=${params.TARGET} (${env.DEPLOY_BRANCH}) build #${env.BUILD_NUMBER} succeeded"
        }
        failure {
            echo "❌ Pipeline failed - ${env.DEPLOY_BRANCH} build #${env.BUILD_NUMBER}"
        }
        always {
            cleanWs(
                cleanWhenAborted: true,
                cleanWhenFailure: true,
                cleanWhenNotBuilt: true,
                cleanWhenSuccess: true,
                cleanWhenUnstable: true,
                deleteDirs: true,
                disableDeferredWipeout: true,
                notFailBuild: true
            )
        }
    }
}
