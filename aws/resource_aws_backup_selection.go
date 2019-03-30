package aws

import (
	"bytes"
	"fmt"
	"log"
	"regexp"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/backup"
	"github.com/hashicorp/terraform/helper/hashcode"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/helper/validation"
)

func resourceAwsBackupSelection() *schema.Resource {
	return &schema.Resource{
		Create: resourceAwsBackupSelectionCreate,
		Read:   resourceAwsBackupSelectionRead,
		Delete: resourceAwsBackupSelectionDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: validation.All(
					validation.StringLenBetween(1, 50),
					validation.StringMatch(regexp.MustCompile(`^[a-zA-Z0-9\-\_\.]+$`), "must contain only alphanumeric, hyphen, underscore, and period characters"),
				),
			},
			"plan_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"iam_role_arn": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validateArn,
			},
			"tag": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"type": {
							Type:     schema.TypeString,
							Required: true,
							ValidateFunc: validation.StringInSlice([]string{
								backup.ConditionTypeStringequals,
							}, false),
						},
						"key": {
							Type:     schema.TypeString,
							Required: true,
						},
						"value": {
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
				Set: resourceAwsConditionTagHash,
			},
			"resources": {
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

func resourceAwsBackupSelectionCreate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).backupconn

	selection := &backup.Selection{
		IamRoleArn:    aws.String(d.Get("iam_role_arn").(string)),
		ListOfTags:    expandBackupConditionTags(d.Get("tag").(*schema.Set).List()),
		Resources:     expandStringList(d.Get("resources").([]interface{})),
		SelectionName: aws.String(d.Get("name").(string)),
	}

	input := &backup.CreateBackupSelectionInput{
		BackupPlanId:    aws.String(d.Get("plan_id").(string)),
		BackupSelection: selection,
	}

	resp, err := conn.CreateBackupSelection(input)
	if err != nil {
		return fmt.Errorf("error creating Backup Selection: %s", err)
	}

	d.SetId(*resp.SelectionId)

	return resourceAwsBackupSelectionRead(d, meta)
}

func resourceAwsBackupSelectionRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).backupconn

	input := &backup.GetBackupSelectionInput{
		BackupPlanId: aws.String(d.Get("plan_id").(string)),
		SelectionId:  aws.String(d.Id()),
	}

	resp, err := conn.GetBackupSelection(input)
	if isAWSErr(err, backup.ErrCodeResourceNotFoundException, "") {
		log.Printf("[WARN] Backup Selection (%s) not found, removing from state", d.Id())
		d.SetId("")
		return nil
	}

	if err != nil {
		return fmt.Errorf("error reading Backup Selection: %s", err)
	}

	d.Set("plan_id", resp.BackupPlanId)
	d.Set("name", resp.BackupSelection.SelectionName)
	d.Set("iam_role", resp.BackupSelection.IamRoleArn)

	if resp.BackupSelection.ListOfTags != nil {
		tag := &schema.Set{F: resourceAwsConditionTagHash}

		for _, r := range resp.BackupSelection.ListOfTags {
			m := make(map[string]interface{})

			m["type"] = aws.StringValue(r.ConditionType)
			m["key"] = aws.StringValue(r.ConditionKey)
			m["value"] = aws.StringValue(r.ConditionValue)

			tag.Add(m)
		}

		if err := d.Set("tag", tag); err != nil {
			return fmt.Errorf("error setting tag: %s", err)
		}
	}
	if resp.BackupSelection.Resources != nil {
		d.Set("resources", resp.BackupSelection.Resources)
	}

	return nil
}

func resourceAwsBackupSelectionDelete(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).backupconn

	input := &backup.DeleteBackupSelectionInput{
		BackupPlanId: aws.String(d.Get("plan_id").(string)),
		SelectionId:  aws.String(d.Id()),
	}

	_, err := conn.DeleteBackupSelection(input)
	if err != nil {
		return fmt.Errorf("error deleting Backup Selection: %s", err)
	}

	return nil
}

func expandBackupConditionTags(tagList []interface{}) []*backup.Condition {
	conditions := []*backup.Condition{}

	for _, i := range tagList {
		item := i.(map[string]interface{})
		tag := &backup.Condition{}

		tag.ConditionType = aws.String(item["type"].(string))
		tag.ConditionKey = aws.String(item["key"].(string))
		tag.ConditionValue = aws.String(item["value"].(string))

		conditions = append(conditions, tag)
	}

	return conditions
}

func resourceAwsConditionTagHash(v interface{}) int {
	var buf bytes.Buffer
	m := v.(map[string]interface{})

	if v, ok := m["type"]; ok {
		buf.WriteString(fmt.Sprintf("%s-", v.(string)))
	}

	if v, ok := m["key"]; ok {
		buf.WriteString(fmt.Sprintf("%s-", v.(string)))
	}

	if v, ok := m["value"]; ok {
		buf.WriteString(fmt.Sprintf("%s-", v.(string)))
	}

	return hashcode.String(buf.String())
}
