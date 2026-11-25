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
	return s.sendTemplate(toEmail, "CloudBin account verification", "user-creation-otp-template.html", map[string]string{"OTP": otpCode})
}

func (s *EmailService) SendForgotPasswordEmail(toEmail, otpCode string) error {
	return s.sendTemplate(toEmail, "CloudBin forgot password", "forgot-password-email-template.html", map[string]string{"OTP": otpCode})
}

func (s *EmailService) SendAccountCreatedEmail(toEmail string) error {
	return s.sendTemplate(toEmail, "Welcome to CloudBin", "account-created-email-template.html", map[string]string{"SupportEmail": s.fromEmail})
}

func (s *EmailService) sendTemplate(toEmail, subject, name string, data map[string]string) error {
	if !s.IsConfigured() {
		return fmt.Errorf("email service is not configured")
	}
	htmlContent, err := s.renderTemplate(name, data)
	if err != nil {
		return err
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
