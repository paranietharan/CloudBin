package email

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"path/filepath"

	"gopkg.in/gomail.v2"
)

//go:embed template/*.html
var templatesFS embed.FS

type EmailService struct {
	smtpHost  string
	smtpPort  int
	smtpUser  string
	smtpPass  string
	fromEmail string
	fromName  string
	templates *template.Template
}

func NewEmailService(smtpHost string, smtpPort int, smtpUser, smtpPass, fromEmail, fromName string) *EmailService {
	if smtpHost == "" {
		smtpHost = "smtp.gmail.com"
	}
	if smtpPort <= 0 {
		smtpPort = 587
	}
	if fromEmail == "" {
		fromEmail = smtpUser
	}
	if fromName == "" {
		fromName = "CloudBin"
	}

	tmpl, _ := template.ParseFS(templatesFS, "template/*.html")

	return &EmailService{
		smtpHost:  smtpHost,
		smtpPort:  smtpPort,
		smtpUser:  smtpUser,
		smtpPass:  smtpPass,
		fromEmail: fromEmail,
		fromName:  fromName,
		templates: tmpl,
	}
}

func (s *EmailService) IsConfigured() bool {
	return s != nil && s.smtpHost != "" && s.smtpPort > 0 && s.smtpUser != "" && s.smtpPass != ""
}

func (s *EmailService) SendAccountCreationOTPEmail(toEmail, otpCode string) error {
	body, err := s.renderTemplate("user-creation-otp-template.html", struct {
		OTP string
	}{OTP: otpCode})
	if err != nil {
		return err
	}
	return s.send(toEmail, "CloudBin account verification", body)
}

func (s *EmailService) SendForgotPasswordEmail(toEmail, otpCode string) error {
	body, err := s.renderTemplate("forgot-password-email-template.html", struct {
		OTP string
	}{OTP: otpCode})
	if err != nil {
		return err
	}
	return s.send(toEmail, "CloudBin forgot password", body)
}

func (s *EmailService) SendAccountCreatedEmail(toEmail string) error {
	body, err := s.renderTemplate("account-created-email-template.html", struct {
		SupportEmail string
	}{SupportEmail: s.fromEmail})
	if err != nil {
		return err
	}
	return s.send(toEmail, "Welcome to CloudBin", body)
}

func (s *EmailService) send(toEmail, subject, htmlContent string) error {
	if !s.IsConfigured() {
		return fmt.Errorf("email service is not configured")
	}

	m := gomail.NewMessage()
	m.SetAddressHeader("From", s.fromEmail, s.fromName)
	m.SetHeader("To", toEmail)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", htmlContent)

	d := gomail.NewDialer(s.smtpHost, s.smtpPort, s.smtpUser, s.smtpPass)
	if err := d.DialAndSend(m); err != nil {
		return fmt.Errorf("send smtp email: %w", err)
	}

	return nil
}

func (s *EmailService) renderTemplate(name string, data any) (string, error) {
	var out bytes.Buffer
	if s.templates != nil {
		if err := s.templates.ExecuteTemplate(&out, name, data); err == nil {
			return out.String(), nil
		}
	}

	// Fallback to direct parse if template not found in bundle
	tmpl, err := template.ParseFS(templatesFS, filepath.Join("template", name))
	if err != nil {
		return "", err
	}
	if err := tmpl.Execute(&out, data); err != nil {
		return "", err
	}
	return out.String(), nil
}
