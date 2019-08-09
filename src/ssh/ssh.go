package ssh

import (
	"fmt"
	"io"
	"meta"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

type SSHTerminal struct {
	Session *ssh.Session
	exitMsg string
	stdout  io.Reader
	stdin   io.Writer
	stderr  io.Reader
}

func NewSSHConnection(node *meta.Node) error {
	sshConfig := &ssh.ClientConfig{
		User: node.UserName,
		Auth: []ssh.AuthMethod{
			ssh.Password(node.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	addrInfo := fmt.Sprintf("%s:%d", node.Ip, node.Port)
	client, err := ssh.Dial("tcp", addrInfo, sshConfig)
	if err != nil {
		return err
	}
	defer client.Close()

	err = NewSShClient(client)
	if err != nil {
		return err
	}
	return nil
}

func Run(node *meta.Node, cmd string) ([]byte, error) {
	sshConfig := &ssh.ClientConfig{
		User: node.UserName,
		Auth: []ssh.AuthMethod{
			ssh.Password(node.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	addrInfo := fmt.Sprintf("%s:%d", node.Ip, node.Port)
	client, err := ssh.Dial("tcp", addrInfo, sshConfig)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()
	combo, err := session.CombinedOutput(cmd)
	if err != nil {
		return nil, err
	}
	return combo, nil
}
func (t *SSHTerminal) updateTerminalSize() {

	go func() {
		// SIGWINCH is sent to the process when the window size of the terminal has
		// changed.
		sigwinchCh := make(chan os.Signal, 1)
		signal.Notify(sigwinchCh, syscall.SIGWINCH)

		fd := int(os.Stdin.Fd())
		termWidth, termHeight, err := terminal.GetSize(fd)
		if err != nil {
			fmt.Println(err)
		}

		for {
			select {
			// The client updated the size of the local PTY. This change needs to occur
			// on the server side PTY as well.
			case sigwinch := <-sigwinchCh:
				if sigwinch == nil {
					return
				}
				currTermWidth, currTermHeight, err := terminal.GetSize(fd)

				// Terminal size has not changed, don't do anything.
				if currTermHeight == termHeight && currTermWidth == termWidth {
					continue
				}

				t.Session.WindowChange(currTermHeight, currTermWidth)

				if err != nil {
					fmt.Printf("Unable to send window-change reqest: %s.", err)
					continue
				}

				termWidth, termHeight = currTermWidth, currTermHeight

			}
		}
	}()

}

func (t *SSHTerminal) interactiveSession() error {

	defer func() {
		if t.exitMsg == "" {
			fmt.Fprintln(os.Stdout, "the connection was closed on the remote side on ", time.Now().Format(time.RFC822))
		} else {
			fmt.Fprintln(os.Stdout, t.exitMsg)
		}
	}()

	fd := int(os.Stdin.Fd())
	state, err := terminal.MakeRaw(fd)
	if err != nil {
		return err
	}
	defer terminal.Restore(fd, state)

	termWidth, termHeight, err := terminal.GetSize(fd)
	if err != nil {
		return err
	}

	termType := os.Getenv("TERM")
	if termType == "" {
		termType = "xterm-256color"
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	err = t.Session.RequestPty(termType, termHeight, termWidth, modes)
	if err != nil {
		return err
	}

	t.updateTerminalSize()

	t.stdin, err = t.Session.StdinPipe()
	//t.stdin, err = os.Stdin
	if err != nil {
		return err
	}
	t.stdout, err = t.Session.StdoutPipe()
	if err != nil {
		return err
	}
	t.stderr, err = t.Session.StderrPipe()

	go io.Copy(os.Stderr, t.stderr)
	go io.Copy(os.Stdout, t.stdout)

	go func() {
		buf := make([]byte, 2048)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				fmt.Println(err)
				return
			}
			if n > 0 {
				_, err = t.stdin.Write(buf[:n])
				if err != nil {
					fmt.Println(err)
					t.exitMsg = err.Error()
					return
				}
			}
		}
	}()
	err = t.Session.Shell()
	if err != nil {
		return err
	}
	err = t.Session.Wait()
	if err != nil {
		return err
	}
	return nil
}

func NewSShClient(client *ssh.Client) error {

	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	s := SSHTerminal{
		Session: session,
	}

	return s.interactiveSession()
}
