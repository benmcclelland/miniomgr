package backend

import (
	"bufio"
	"context"
	"fmt"
	"hash/crc32"
	"log"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"strconv"
	"syscall"
	"time"

	"github.com/benmcclelland/miniomgr/db"
)

var (
	//TODO make this a config
	startport = 9000
	pwRxp     = regexp.MustCompile(`([^:]+):([^:]*):([^:]*):([^:]*):([^:]*):([^:]*):([^:]*)`)
)

type process struct {
	cmd    *exec.Cmd
	port   int
	cancel context.CancelFunc
}

type MinioMgr struct {
	db         db.DB
	exportPath string
	nextport   int
	procs      map[string]process
}

func New(path string) (*MinioMgr, error) {
	bdb, err := db.NewBoltDB()
	if err != nil {
		return nil, err
	}

	m := &MinioMgr{
		db:         bdb,
		exportPath: path,
		nextport:   startport,
		procs:      make(map[string]process),
	}

	err = m.populateUsers()
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (m *MinioMgr) GetUserURL(user string) (string, error) {
	if val, ok := m.procs[user]; ok {
		return fmt.Sprintf("%v", val.port), nil
	}

	port := m.nextport

	cmd, cancel, err := m.cmdByUser(user)
	if err != nil {
		return "", err
	}

	err = m.runAsUserName(user, cmd)
	if err != nil {
		return "", err
	}
	m.nextport++

	p := process{
		cmd:    cmd,
		port:   port,
		cancel: cancel,
	}

	m.procs[user] = p

	time.Sleep(1 * time.Second)

	return fmt.Sprintf("%v", port), nil
}

func (m *MinioMgr) Done() {
	for _, p := range m.procs {
		p.cancel()

		cancel := make(chan struct{}, 1)
		done := make(chan struct{}, 1)

		t := time.AfterFunc(5*time.Second, func() {
			close(cancel)
		})
		defer t.Stop()

		go func() {
			err := p.cmd.Wait()
			if err != nil {
				log.Printf("wait on pid %v returned %v", p.cmd.Process.Pid, err)
			}
			close(done)
		}()

		select {
		case <-cancel:
			log.Printf("could not kill server listening on %v pid: %v", p.port, p.cmd.Process.Pid)
			return
		case <-done:
			return
		}
	}
}

func (m *MinioMgr) runAsUserName(username string, cmd *exec.Cmd) error {
	myuser, err := user.Lookup(username)
	if err != nil {
		return err
	}

	uid, _ := strconv.Atoi(myuser.Uid)
	if err != nil {
		return err
	}

	gid, _ := strconv.Atoi(myuser.Gid)
	if err != nil {
		return err
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{}
	cmd.SysProcAttr.Credential = &syscall.Credential{
		Uid: uint32(uid),
		Gid: uint32(gid),
	}

	return cmd.Start()
}

func (m *MinioMgr) cmdByUser(username string) (*exec.Cmd, context.CancelFunc, error) {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "minio", "gateway", "--address",
		fmt.Sprintf(":%v", m.nextport), "nas", m.exportPath)

	accesskey, secretkey, err := m.db.GetKeys(username)
	if err != nil {
		return nil, cancel, err
	}
	cmd.Env = append(os.Environ(),
		"MINIO_ACCESS_KEY="+accesskey,
		"MINIO_SECRET_KEY="+secretkey,
		"HOME=/home/"+username,
	)

	log.Println("MINIO_ACCESS_KEY="+accesskey, "MINIO_SECRET_KEY="+secretkey, "HOME=/home/"+username)

	return cmd, cancel, nil
}

func (m *MinioMgr) populateUsers() error {
	f, err := os.Open("/etc/passwd")

	if err != nil {
		return err
	}

	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		// username:password:uid:gid:gecos:home:shell
		// root:x:0:0:root:/root:/bin/bash
		match := pwRxp.FindStringSubmatch(s.Text())
		if len(match) < 8 {
			log.Printf("skipping bad entry %q", s.Text())
			continue
		}
		username := match[1]
		key := fmt.Sprintf("%08x", crc32.ChecksumIEEE([]byte(username)))
		err = m.db.SetKeys(username, username, key)
		if err != nil {
			log.Printf("error updating db keys for %v: %v", username, err)
			continue
		}
		log.Printf("user: %v key: %v", username, key)
	}
	if err := s.Err(); err != nil {
		return fmt.Errorf("could not scan: %v", err)
	}

	return nil
}
