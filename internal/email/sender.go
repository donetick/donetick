package email

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"donetick.com/core/config"
	gomail "gopkg.in/gomail.v2"
)

type EmailSender struct {
	client    *gomail.Dialer
	appHost   string
	fromEmail string
}

func NewEmailSender(conf *config.Config) *EmailSender {
	var client *gomail.Dialer

	// Use username if provided, otherwise fall back to email for backwards compatibility
	var emailIdentifier string

	if conf.EmailConfig.Username != "" {
		emailIdentifier = conf.EmailConfig.Username
	} else if conf.EmailConfig.Email != "" {
		emailIdentifier = conf.EmailConfig.Email
	} else {
		return nil
	}

	client = gomail.NewDialer(
		conf.EmailConfig.Host,
		conf.EmailConfig.Port,
		emailIdentifier,
		conf.EmailConfig.Key)

	// format conf.EmailConfig.Host and port :

	// auth := smtp.PlainAuth("", conf.EmailConfig.Email, conf.EmailConfig.Password, host)
	return &EmailSender{
		client:    client,
		appHost:   conf.EmailConfig.AppHost,
		fromEmail: conf.EmailConfig.Email,
	}
}

func (es *EmailSender) SendVerificationEmail(to, code string) error {
	// msg := []byte(fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s\r\n", to, subject, body))
	msg := gomail.NewMessage()
	msg.SetHeader("From", es.fromEmail)
	msg.SetHeader("To", to)
	msg.SetHeader("Subject", "Welcome to Donetick! Verify your email")
	// text/html for a html email
	htmlBody := `
	<!--
	********************************************************
	* This email was built using Tabular.
	* Create emails, that look perfect in every inbox.
	* For more information, visit https://tabular.email
	********************************************************
	-->
	<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
	<html xmlns="http://www.w3.org/1999/xhtml" xmlns:v="urn:schemas-microsoft-com:vml" xmlns:o="urn:schemas-microsoft-com:office:office" lang="en"><head>
		<title></title>
		<meta content="text/html; charset=utf-8" http-equiv="Content-Type">
		<!--[if !mso]><!-->
		<meta http-equiv="X-UA-Compatible" content="IE=edge">
		<!--<![endif]-->
		<meta name="x-apple-disable-message-reformatting" content="">
		<meta content="target-densitydpi=device-dpi" name="viewport">
		<meta content="true" name="HandheldFriendly">
		<meta content="width=device-width" name="viewport">
		<meta name="format-detection" content="telephone=no, date=no, address=no, email=no, url=no">
		<style type="text/css">
		table {
		border-collapse: separate;
		table-layout: fixed;
		mso-table-lspace: 0pt;
		mso-table-rspace: 0pt
		}
		table td {
		border-collapse: collapse
		}
		.ExternalClass {
		width: 100%
		}
		.ExternalClass,
		.ExternalClass p,
		.ExternalClass span,
		.ExternalClass font,
		.ExternalClass td,
		.ExternalClass div {
		line-height: 100%
		}
		body, a, li, p, h1, h2, h3 {
		-ms-text-size-adjust: 100%;
		-webkit-text-size-adjust: 100%;
		}
		html {
		-webkit-text-size-adjust: none !important
		}
		body, #innerTable {
		-webkit-font-smoothing: antialiased;
		-moz-osx-font-smoothing: grayscale
		}
		#innerTable img+div {
		display: none;
		display: none !important
		}
		img {
		Margin: 0;
		padding: 0;
		-ms-interpolation-mode: bicubic
		}
		h1, h2, h3, p, a {
		line-height: 1;
		overflow-wrap: normal;
		white-space: normal;
		word-break: break-word
		}
		a {
		text-decoration: none
		}
		h1, h2, h3, p {
		min-width: 100%!important;
		width: 100%!important;
		max-width: 100%!important;
		display: inline-block!important;
		border: 0;
		padding: 0;
		margin: 0
		}
		a[x-apple-data-detectors] {
		color: inherit !important;
		text-decoration: none !important;
		font-size: inherit !important;
		font-family: inherit !important;
		font-weight: inherit !important;
		line-height: inherit !important
		}
		a[href^="mailto"],
		a[href^="tel"],
		a[href^="sms"] {
		color: inherit;
		text-decoration: none
		}
		@media (min-width: 481px) {
		.hd { display: none!important }
		}
		@media (max-width: 480px) {
		.hm { display: none!important }
		}
		[style*="Inter Tight"] {font-family: 'Inter Tight', BlinkMacSystemFont,Segoe UI,Helvetica Neue,Arial,sans-serif !important;} [style*="Albert Sans"] {font-family: 'Albert Sans', BlinkMacSystemFont,Segoe UI,Helvetica Neue,Arial,sans-serif !important;}
		@media only screen and (min-width: 481px) {.t20{width:720px!important}.t27{padding:40px 60px 50px!important}.t29{padding:40px 60px 50px!important;width:680px!important}.t43{width:600px!important}.t53,.t61{width:580px!important}.t65{width:600px!important}.t78{padding-left:0!important;padding-right:0!important}.t80{padding-left:0!important;padding-right:0!important;width:400px!important}.t84,.t94{width:600px!important}}
		</style>
		<style type="text/css" media="screen and (min-width:481px)">.moz-text-html .t20{width:720px!important}.moz-text-html .t27{padding:40px 60px 50px!important}.moz-text-html .t29{padding:40px 60px 50px!important;width:680px!important}.moz-text-html .t43{width:600px!important}.moz-text-html .t53,.moz-text-html .t61{width:580px!important}.moz-text-html .t65{width:600px!important}.moz-text-html .t78{padding-left:0!important;padding-right:0!important}.moz-text-html .t80{padding-left:0!important;padding-right:0!important;width:400px!important}.moz-text-html .t84,.moz-text-html .t94{width:600px!important}</style>
		<!--[if !mso]><!-->
		<link href="https://fonts.googleapis.com/css2?family=Inter+Tight:wght@400;600&amp;family=Albert+Sans:wght@800&amp;display=swap" rel="stylesheet" type="text/css">
		<!--<![endif]-->
		<!--[if mso]>
		<style type="text/css">
		td.t20{width:800px !important}td.t27{padding:40px 60px 50px !important}td.t29{padding:40px 60px 50px !important;width:800px !important}td.t43,td.t53{width:600px !important}td.t61{width:580px !important}td.t65{width:600px !important}td.t78,td.t80{padding-left:0 !important;padding-right:0 !important}td.t84,td.t94{width:600px !important}
		</style>
		<![endif]-->
		<!--[if mso]>
		<xml>
		<o:OfficeDocumentSettings>
		<o:AllowPNG/>
		<o:PixelsPerInch>96</o:PixelsPerInch>
		</o:OfficeDocumentSettings>
		</xml>
		<![endif]-->
		</head>
		<body class="t0" style="min-width:100%;Marg
		if you did not sign up with Donetick please Ignore this email. in:0px;padding:0px;background-color:#FFFFFF;"><div class="t1" style="background-color:#FFFFFF;"><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" align="center"><tbody><tr><td class="t130" style="font-size:0;line-height:0;mso-line-height-rule:exactly;" valign="top" align="center">
		<!--[if mso]>
		<v:background xmlns:v="urn:schemas-microsoft-com:vml" fill="true" stroke="false">
		<v:fill color="#FFFFFF"/>
		</v:background>
		<![endif]-->
		<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" align="center" id="innerTable"><tbody><tr><td>
		<table class="t118" role="presentation" cellpadding="0" cellspacing="0" align="center"><tbody><tr>
	
		<!--<![endif]-->
		<!--[if mso]><td class="t119" style="width:400px;padding:40px 40px 40px 40px;"><![endif]-->
		</tr></tbody></table><table role="presentation" width="100%" cellpadding="0" cellspacing="0"></table></td>
		</tr></tbody></table>
		</td></tr><tr><td>
		<table class="t10" role="presentation" cellpadding="0" cellspacing="0" align="center"><tbody><tr>
		<!--[if !mso]><!--><td class="t11" style="background-color:#FFFFFF;width:400px;">
		<!--<![endif]-->
		<!--[if mso]><td class="t11" style="background-color:#FFFFFF;width:400px;"><![endif]-->
		<table role="presentation" width="100%" cellpadding="0" cellspacing="0"><tbody><tr><td>
		<table class="t19" role="presentation" cellpadding="0" cellspacing="0" align="center"><tbody><tr>
		<!--[if !mso]><!--><td class="t20" style="background-color:#404040;width:400px;padding:40px 40px 40px 40px;">
		<!--<![endif]-->
		<!--[if mso]><td class="t20" style="background-color:#404040;width:480px;padding:40px 40px 40px 40px;"><![endif]-->
		<table role="presentation" width="100%" cellpadding="0" cellspacing="0"><tbody><tr><td>
		<table class="t103" role="presentation" cellpadding="0" cellspacing="0" align="center"><tbody><tr>
		<!--[if !mso]><!--><td class="t104" style="width:200px;">
		<!--<![endif]-->
		<!--[if mso]><td class="t104" style="width:200px;"><![endif]-->
		<div style="font-size:0px;"><img class="t110" style="display:block;border:0;height:auto;width:100%;Margin:0;max-width:100%;" width="200" height="179.5" alt="" src="https://835a1b8e-557a-4713-8f1c-104febdb8808.b-cdn.net/e/30b4288c-4e67-4e3b-9527-1fc4c4ec2fdf/df3f012a-c060-4d59-b5fd-54a57dae1916.png"></div></td>
		</tr></tbody></table>
		</td></tr></tbody></table></td>
		</tr></tbody></table>
		</td></tr><tr><td>
		<table class="t28" role="presentation" cellpadding="0" cellspacing="0" align="center"><tbody><tr>
		<!--[if !mso]><!--><td class="t29" style="width:420px;padding:30px 30px 40px 30px;">
		<!--<![endif]-->
		<!--[if mso]><td class="t29" style="width:480px;padding:30px 30px 40px 30px;"><![endif]-->
		<table role="presentation" width="100%" cellpadding="0" cellspacing="0"><tbody><tr><td>
		<table class="t42" role="presentation" cellpadding="0" cellspacing="0" align="center"><tbody><tr>
		<!--[if !mso]><!--><td class="t43" style="width:480px;">
		<!--<![endif]-->
		<!--[if mso]><td class="t43" style="width:480px;"><![endif]-->
		<h1 class="t49" style="margin:0;Margin:0;font-family:BlinkMacSystemFont,Segoe UI,Helvetica Neue,Arial,sans-serif,'Albert Sans';line-height:35px;font-weight:800;font-style:normal;font-size:30px;text-decoration:none;text-transform:none;letter-spacing:-1.2px;direction:ltr;color:#333333;text-align:center;mso-line-height-rule:exactly;mso-text-raise:2px;">Welcome to Donetick!</h1></td>
		</tr></tbody></table>
		</td></tr><tr><td><div class="t41" style="mso-line-height-rule:exactly;mso-line-height-alt:16px;line-height:16px;font-size:1px;display:block;">&nbsp;</div></td></tr><tr><td><p class="t39" style="margin:0;Margin:0;font-family:BlinkMacSystemFont,Segoe UI,Helvetica Neue,Arial,sans-serif,'Inter Tight';line-height:21px;font-weight:400;font-style:normal;font-size:14px;text-decoration:none;text-transform:none;direction:ltr;color:#555555;text-align:center;mso-line-height-rule:exactly;mso-text-raise:2px;">Thank you for joining us. We're excited to have you on board.To complete your registration, click the button below</p></td></tr><tr><td><div class="t31" style="mso-line-height-rule:exactly;mso-line-height-alt:30px;line-height:30px;font-size:1px;display:block;">&nbsp;</div></td></tr><tr><td>
		<table class="t52" role="presentation" cellpadding="0" cellspacing="0" align="center"><tbody><tr>
		<!--[if !mso]><!--><td class="t53" style="background-color:#06b6d4;overflow:hidden;width:460px;text-align:center;line-height:24px;mso-line-height-rule:exactly;mso-text-raise:2px;padding:10px 10px 10px 10px;border-radius:10px 10px 10px 10px;">
		<!--<![endif]-->
		<!--[if mso]><td class="t53" style="background-color:#06b6d4;overflow:hidden;width:480px;text-align:center;line-height:24px;mso-line-height-rule:exactly;mso-text-raise:2px;padding:10px 10px 10px 10px;border-radius:10px 10px 10px 10px;"><![endif]-->
		<a class="t59" href="{{verifyURL}}" style="display:block;margin:0;Margin:0;font-family:BlinkMacSystemFont,Segoe UI,Helvetica Neue,Arial,sans-serif,'Inter Tight';line-height:24px;font-weight:600;font-style:normal;font-size:16px;text-decoration:none;direction:ltr;color:#FFFFFF;text-align:center;mso-line-height-rule:exactly;mso-text-raise:2px;" target="_blank">Complete your registration</a></td>
		</tr></tbody></table>
		</td></tr><tr><td><div class="t62" style="mso-line-height-rule:exactly;mso-line-height-alt:12px;line-height:12px;font-size:1px;display:block;">&nbsp;</div></td></tr><tr><td>
		<table class="t64" role="presentation" cellpadding="0" cellspacing="0" align="center"><tbody><tr>
		<!--[if !mso]><!--><td class="t65" style="width:480px;">
		<!--<![endif]-->
		<!--[if mso]><td class="t65" style="width:480px;"><![endif]-->
		<p class="t71" style="margin:0;Margin:0;font-family:BlinkMacSystemFont,Segoe UI,Helvetica Neue,Arial,sans-serif,'Inter Tight';line-height:21px;font-weight:400;font-style:normal;font-size:14px;text-decoration:none;text-transform:none;direction:ltr;color:#555555;text-align:center;mso-line-height-rule:exactly;mso-text-raise:2px;">&nbsp;</p></td>
		</tr></tbody></table>
		</td></tr></tbody></table></td>
		</tr></tbody></table>
		</td></tr></tbody></table></td>
		</tr></tbody></table>
		</td></tr><tr><td><div class="t4" style="mso-line-height-rule:exactly;mso-line-height-alt:30px;line-height:30px;font-size:1px;display:block;">&nbsp;</div></td></tr><tr><td>
		<table class="t79" role="presentation" cellpadding="0" cellspacing="0" align="center"><tbody><tr>
		<!--[if !mso]><!--><td class="t80" style="width:320px;padding:0 40px 0 40px;">
		<!--<![endif]-->
		<!--[if mso]><td class="t80" style="width:400px;padding:0 40px 0 40px;"><![endif]-->
		<table role="presentation" width="100%" cellpadding="0" cellspacing="0"><tbody><tr><td>
		<table class="t93" role="presentation" cellpadding="0" cellspacing="0" align="center"><tbody><tr>
		<!--[if !mso]><!--><td class="t94" style="width:480px;">
		<!--<![endif]-->
		<!--[if mso]><td class="t94" style="width:480px;"><![endif]-->
		<p class="t100" style="margin:0;Margin:0;font-family:BlinkMacSystemFont,Segoe UI,Helvetica Neue,Arial,sans-serif,'Inter Tight';line-height:18px;font-weight:400;font-style:normal;font-size:12px;text-decoration:none;text-transform:none;direction:ltr;color:#555555;text-align:center;mso-line-height-rule:exactly;mso-text-raise:2px;">if you did not sign up with Donetick please Ignore this email. </p></td>
		</tr></tbody></table>
		</td></tr><tr><td><div class="t81" style="mso-line-height-rule:exactly;mso-line-height-alt:8px;line-height:8px;font-size:1px;display:block;">&nbsp;</div></td></tr><tr><td>
		<table class="t83" role="presentation" cellpadding="0" cellspacing="0" align="center"><tbody><tr>
		<!--[if !mso]><!--><td class="t84" style="width:480px;">
		<!--<![endif]-->
		<!--[if mso]><td class="t84" style="width:480px;"><![endif]-->
		<p class="t90" style="margin:0;Margin:0;font-family:BlinkMacSystemFont,Segoe UI,Helvetica Neue,Arial,sans-serif,'Inter Tight';line-height:18px;font-weight:400;font-style:normal;font-size:12px;text-decoration:none;text-transform:none;direction:ltr;color:#555555;text-align:center;mso-line-height-rule:exactly;mso-text-raise:2px;">Favoro LLC. All rights reserved</p></td>
		</tr></tbody></table>
		</td></tr></tbody></table></td>
		</tr></tbody></table>
		</td></tr><tr><td><div class="t73" style="mso-line-height-rule:exactly;mso-line-height-alt:100px;line-height:100px;font-size:1px;display:block;">&nbsp;</div></td></tr></tbody></table></div>
		
	</body></html>
`
	u := es.appHost + "/verify?c=" + encodeEmailAndCode(to, code)
	htmlBody = strings.Replace(htmlBody, "{{verifyURL}}", u, 1)

	msg.SetBody("text/html", htmlBody)

	err := es.client.DialAndSend(msg)
	if err != nil {
		return err
	}
	return nil

}

func (es *EmailSender) SendResetPasswordEmail(c context.Context, to, code string) error {
	msg := gomail.NewMessage()
	msg.SetHeader("From", es.fromEmail)
	msg.SetHeader("To", to)
	msg.SetHeader("Subject", "Donetick! Password Reset")
	htmlBody := `
	<!--
	********************************************************
	* This email was built using Tabular.
	* Create emails, that look perfect in every inbox.
	* For more information, visit https://tabular.email
	********************************************************
	-->
	<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
	<html xmlns="http://www.w3.org/1999/xhtml" xmlns:v="urn:schemas-microsoft-com:vml" xmlns:o="urn:schemas-microsoft-com:office:office" lang="en"><head>
		<title></title>
		<meta content="text/html; charset=utf-8" http-equiv="Content-Type">
		<!--[if !mso]><!-->
		<meta http-equiv="X-UA-Compatible" content="IE=edge">
		<!--<![endif]-->
		<meta name="x-apple-disable-message-reformatting" content="">
		<meta content="target-densitydpi=device-dpi" name="viewport">
		<meta content="true" name="HandheldFriendly">
		<meta content="width=device-width" name="viewport">
		<meta name="format-detection" content="telephone=no, date=no, address=no, email=no, url=no">
		<style type="text/css">
		table {
		border-collapse: separate;
		table-layout: fixed;
		mso-table-lspace: 0pt;
		mso-table-rspace: 0pt
		}
		table td {
		border-collapse: collapse
		}
		.ExternalClass {
		width: 100%
		}
		.ExternalClass,
		.ExternalClass p,
		.ExternalClass span,
		.ExternalClass font,
		.ExternalClass td,
		.ExternalClass div {
		line-height: 100%
		}
		body, a, li, p, h1, h2, h3 {
		-ms-text-size-adjust: 100%;
		-webkit-text-size-adjust: 100%;
		}
		html {
		-webkit-text-size-adjust: none !important
		}
		body, #innerTable {
		-webkit-font-smoothing: antialiased;
		-moz-osx-font-smoothing: grayscale
		}
		#innerTable img+div {
		display: none;
		display: none !important
		}
		img {
		Margin: 0;
		padding: 0;
		-ms-interpolation-mode: bicubic
		}
		h1, h2, h3, p, a {
		line-height: 1;
		overflow-wrap: normal;
		white-space: normal;
		word-break: break-word
		}
		a {
		text-decoration: none
		}
		h1, h2, h3, p {
		min-width: 100%!important;
		width: 100%!important;
		max-width: 100%!important;
		display: inline-block!important;
		border: 0;
		padding: 0;
		margin: 0
		}
		a[x-apple-data-detectors] {
		color: inherit !important;
		text-decoration: none !important;
		font-size: inherit !important;
		font-family: inherit !important;
		font-weight: inherit !important;
		line-height: inherit !important
		}
		a[href^="mailto"],
		a[href^="tel"],
		a[href^="sms"] {
		color: inherit;
		text-decoration: none
		}
		@media (min-width: 481px) {
		.hd { display: none!important }
		}
		@media (max-width: 480px) {
		.hm { display: none!important }
		}
		[style*="Inter Tight"] {font-family: 'Inter Tight', BlinkMacSystemFont,Segoe UI,Helvetica Neue,Arial,sans-serif !important;} [style*="Albert Sans"] {font-family: 'Albert Sans', BlinkMacSystemFont,Segoe UI,Helvetica Neue,Arial,sans-serif !important;}
		@media only screen and (min-width: 481px) {.t20{width:720px!important}.t27{padding:40px 60px 50px!important}.t29{padding:40px 60px 50px!important;width:680px!important}.t43{width:600px!important}.t53,.t61{width:580px!important}.t65{width:600px!important}.t78{padding-left:0!important;padding-right:0!important}.t80{padding-left:0!important;padding-right:0!important;width:400px!important}.t84,.t94{width:600px!important}}
		</style>
		<style type="text/css" media="screen and (min-width:481px)">.moz-text-html .t20{width:720px!important}.moz-text-html .t27{padding:40px 60px 50px!important}.moz-text-html .t29{padding:40px 60px 50px!important;width:680px!important}.moz-text-html .t43{width:600px!important}.moz-text-html .t53,.moz-text-html .t61{width:580px!important}.moz-text-html .t65{width:600px!important}.moz-text-html .t78{padding-left:0!important;padding-right:0!important}.moz-text-html .t80{padding-left:0!important;padding-right:0!important;width:400px!important}.moz-text-html .t84,.moz-text-html .t94{width:600px!important}</style>
		<!--[if !mso]><!-->
		<link href="https://fonts.googleapis.com/css2?family=Inter+Tight:wght@400;600&amp;family=Albert+Sans:wght@800&amp;display=swap" rel="stylesheet" type="text/css">
		<!--<![endif]-->
		<!--[if mso]>
		<style type="text/css">
		td.t20{width:800px !important}td.t27{padding:40px 60px 50px !important}td.t29{padding:40px 60px 50px !important;width:800px !important}td.t43,td.t53{width:600px !important}td.t61{width:580px !important}td.t65{width:600px !important}td.t78,td.t80{padding-left:0 !important;padding-right:0 !important}td.t84,td.t94{width:600px !important}
		</style>
		<![endif]-->
		<!--[if mso]>
		<xml>
		<o:OfficeDocumentSettings>
		<o:AllowPNG/>
		<o:PixelsPerInch>96</o:PixelsPerInch>
		</o:OfficeDocumentSettings>
		</xml>
		<![endif]-->
		</head>
		<body class="t0" style="min-width:100%;Marg
		if you did not sign up with Donetick please Ignore this email. in:0px;padding:0px;background-color:#FFFFFF;"><div class="t1" style="background-color:#FFFFFF;"><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" align="center"><tbody><tr><td class="t130" style="font-size:0;line-height:0;mso-line-height-rule:exactly;" valign="top" align="center">
		<!--[if mso]>
		<v:background xmlns:v="urn:schemas-microsoft-com:vml" fill="true" stroke="false">
		<v:fill color="#FFFFFF"/>
		</v:background>
		<![endif]-->
		<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" align="center" id="innerTable"><tbody><tr><td>
		<table class="t118" role="presentation" cellpadding="0" cellspacing="0" align="center"><tbody><tr>
	
		<!--<![endif]-->
		<!--[if mso]><td class="t119" style="width:400px;padding:40px 40px 40px 40px;"><![endif]-->
		</tr></tbody></table><table role="presentation" width="100%" cellpadding="0" cellspacing="0"></table></td>
		</tr></tbody></table>
		</td></tr><tr><td>
		<table class="t10" role="presentation" cellpadding="0" cellspacing="0" align="center"><tbody><tr>
		<!--[if !mso]><!--><td class="t11" style="background-color:#FFFFFF;width:400px;">
		<!--<![endif]-->
		<!--[if mso]><td class="t11" style="background-color:#FFFFFF;width:400px;"><![endif]-->
		<table role="presentation" width="100%" cellpadding="0" cellspacing="0"><tbody><tr><td>
		<table class="t19" role="presentation" cellpadding="0" cellspacing="0" align="center"><tbody><tr>
		<!--[if !mso]><!--><td class="t20" style="background-color:#404040;width:400px;padding:40px 40px 40px 40px;">
		<!--<![endif]-->
		<!--[if mso]><td class="t20" style="background-color:#404040;width:480px;padding:40px 40px 40px 40px;"><![endif]-->
		<table role="presentation" width="100%" cellpadding="0" cellspacing="0"><tbody><tr><td>
		<table class="t103" role="presentation" cellpadding="0" cellspacing="0" align="center"><tbody><tr>
		<!--[if !mso]><!--><td class="t104" style="width:200px;">
		<!--<![endif]-->
		<!--[if mso]><td class="t104" style="width:200px;"><![endif]-->
		<div style="font-size:0px;"><img class="t110" style="display:block;border:0;height:auto;width:100%;Margin:0;max-width:100%;" width="200" height="179.5" alt="" src="https://835a1b8e-557a-4713-8f1c-104febdb8808.b-cdn.net/e/30b4288c-4e67-4e3b-9527-1fc4c4ec2fdf/df3f012a-c060-4d59-b5fd-54a57dae1916.png"></div></td>
		</tr></tbody></table>
		</td></tr></tbody></table></td>
		</tr></tbody></table>
		</td></tr><tr><td>
		<table class="t28" role="presentation" cellpadding="0" cellspacing="0" align="center"><tbody><tr>
		<!--[if !mso]><!--><td class="t29" style="width:420px;padding:30px 30px 40px 30px;">
		<!--<![endif]-->
		<!--[if mso]><td class="t29" style="width:480px;padding:30px 30px 40px 30px;"><![endif]-->
		<table role="presentation" width="100%" cellpadding="0" cellspacing="0"><tbody><tr><td>
		<table class="t42" role="presentation" cellpadding="0" cellspacing="0" align="center"><tbody><tr>
		<!--[if !mso]><!--><td class="t43" style="width:480px;">
		<!--<![endif]-->
		<!--[if mso]><td class="t43" style="width:480px;"><![endif]-->
		<h1 class="t49" style="margin:0;Margin:0;font-family:BlinkMacSystemFont,Segoe UI,Helvetica Neue,Arial,sans-serif,'Albert Sans';line-height:35px;font-weight:800;font-style:normal;font-size:30px;text-decoration:none;text-transform:none;letter-spacing:-1.2px;direction:ltr;color:#333333;text-align:center;mso-line-height-rule:exactly;mso-text-raise:2px;">Someone forgot their password 😔</h1></td>
		</tr></tbody></table>
		</td></tr><tr><td><div class="t41" style="mso-line-height-rule:exactly;mso-line-height-alt:16px;line-height:16px;font-size:1px;display:block;">&nbsp;</div></td></tr><tr><td><p class="t39" style="margin:0;Margin:0;font-family:BlinkMacSystemFont,Segoe UI,Helvetica Neue,Arial,sans-serif,'Inter Tight';line-height:21px;font-weight:400;font-style:normal;font-size:14px;text-decoration:none;text-transform:none;direction:ltr;color:#555555;text-align:center;mso-line-height-rule:exactly;mso-text-raise:2px;">We have received a password reset request for this email address. If you initiated this request, please click the button below to reset your password. Otherwise, you may safely ignore this email.</p></td></tr><tr><td><div class="t31" style="mso-line-height-rule:exactly;mso-line-height-alt:30px;line-height:30px;font-size:1px;display:block;">&nbsp;</div></td></tr><tr><td>
		<table class="t52" role="presentation" cellpadding="0" cellspacing="0" align="center"><tbody><tr>
		<!--[if !mso]><!--><td class="t53" style="background-color:#06b6d4;overflow:hidden;width:460px;text-align:center;line-height:24px;mso-line-height-rule:exactly;mso-text-raise:2px;padding:10px 10px 10px 10px;border-radius:10px 10px 10px 10px;">
		<!--<![endif]-->
		<!--[if mso]><td class="t53" style="background-color:#06b6d4;overflow:hidden;width:480px;text-align:center;line-height:24px;mso-line-height-rule:exactly;mso-text-raise:2px;padding:10px 10px 10px 10px;border-radius:10px 10px 10px 10px;"><![endif]-->
		<a class="t59" href="{{verifyURL}}" style="display:block;margin:0;Margin:0;font-family:BlinkMacSystemFont,Segoe UI,Helvetica Neue,Arial,sans-serif,'Inter Tight';line-height:24px;font-weight:600;font-style:normal;font-size:16px;text-decoration:none;direction:ltr;color:#FFFFFF;text-align:center;mso-line-height-rule:exactly;mso-text-raise:2px;" target="_blank">Reset your Password</a></td>
		</tr></tbody></table>
		</td></tr><tr><td><div class="t62" style="mso-line-height-rule:exactly;mso-line-height-alt:12px;line-height:12px;font-size:1px;display:block;">&nbsp;</div></td></tr><tr><td>
		<table class="t64" role="presentation" cellpadding="0" cellspacing="0" align="center"><tbody><tr>
		<!--[if !mso]><!--><td class="t65" style="width:480px;">
		<!--<![endif]-->
		<!--[if mso]><td class="t65" style="width:480px;"><![endif]-->
		<p class="t71" style="margin:0;Margin:0;font-family:BlinkMacSystemFont,Segoe UI,Helvetica Neue,Arial,sans-serif,'Inter Tight';line-height:21px;font-weight:400;font-style:normal;font-size:14px;text-decoration:none;text-transform:none;direction:ltr;color:#555555;text-align:center;mso-line-height-rule:exactly;mso-text-raise:2px;">&nbsp;</p></td>
		</tr></tbody></table>
		</td></tr></tbody></table></td>
		</tr></tbody></table>
		</td></tr></tbody></table></td>
		</tr></tbody></table>
		</td></tr><tr><td><div class="t4" style="mso-line-height-rule:exactly;mso-line-height-alt:30px;line-height:30px;font-size:1px;display:block;">&nbsp;</div></td></tr><tr><td>
		<table class="t79" role="presentation" cellpadding="0" cellspacing="0" align="center"><tbody><tr>
		<!--[if !mso]><!--><td class="t80" style="width:320px;padding:0 40px 0 40px;">
		<!--<![endif]-->
		<!--[if mso]><td class="t80" style="width:400px;padding:0 40px 0 40px;"><![endif]-->
		<table role="presentation" width="100%" cellpadding="0" cellspacing="0"><tbody><tr><td>
		<table class="t93" role="presentation" cellpadding="0" cellspacing="0" align="center"><tbody><tr>
		<!--[if !mso]><!--><td class="t94" style="width:480px;">
		<!--<![endif]-->
		<!--[if mso]><td class="t94" style="width:480px;"><![endif]-->
		<p class="t100" style="margin:0;Margin:0;font-family:BlinkMacSystemFont,Segoe UI,Helvetica Neue,Arial,sans-serif,'Inter Tight';line-height:18px;font-weight:400;font-style:normal;font-size:12px;text-decoration:none;text-transform:none;direction:ltr;color:#555555;text-align:center;mso-line-height-rule:exactly;mso-text-raise:2px;">if you did not sign up with Donetick please Ignore this email. </p></td>
		</tr></tbody></table>
		</td></tr><tr><td><div class="t81" style="mso-line-height-rule:exactly;mso-line-height-alt:8px;line-height:8px;font-size:1px;display:block;">&nbsp;</div></td></tr><tr><td>
		<table class="t83" role="presentation" cellpadding="0" cellspacing="0" align="center"><tbody><tr>
		<!--[if !mso]><!--><td class="t84" style="width:480px;">
		<!--<![endif]-->
		<!--[if mso]><td class="t84" style="width:480px;"><![endif]-->
		<p class="t90" style="margin:0;Margin:0;font-family:BlinkMacSystemFont,Segoe UI,Helvetica Neue,Arial,sans-serif,'Inter Tight';line-height:18px;font-weight:400;font-style:normal;font-size:12px;text-decoration:none;text-transform:none;direction:ltr;color:#555555;text-align:center;mso-line-height-rule:exactly;mso-text-raise:2px;">Favoro LLC. All rights reserved</p></td>
		</tr></tbody></table>
		</td></tr></tbody></table></td>
		</tr></tbody></table>
		</td></tr><tr><td><div class="t73" style="mso-line-height-rule:exactly;mso-line-height-alt:100px;line-height:100px;font-size:1px;display:block;">&nbsp;</div></td></tr></tbody></table></div>
		
	</body></html>
`
	u := es.appHost + "/password/update?c=" + encodeEmailAndCode(to, code)

	// logging.FromContext(c).Infof("Reset password URL: %s", u)
	htmlBody = strings.Replace(htmlBody, "{{verifyURL}}", u, 1)

	msg.SetBody("text/html", htmlBody)

	err := es.client.DialAndSend(msg)
	if err != nil {
		return err
	}
	return nil

}

// func (es *EmailSender) SendFeedbackRequestEmail(to, code string) error {
// 	// msg := []byte(fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s\r\n", to, subject, body))
// 	msg := gomail.NewMessage()

// }
func encodeEmailAndCode(email, code string) string {
	data := email + ":" + code
	return base64.StdEncoding.EncodeToString([]byte(data))
}

func DecodeEmailAndCode(encoded string) (string, string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", "", err
	}
	parts := string(data)
	split := strings.Split(parts, ":")
	if len(split) != 2 {
		return "", "", fmt.Errorf("Invalid format")
	}
	return split[0], split[1], nil
}
