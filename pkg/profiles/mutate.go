package profiles

import (
	"fmt"
	"strings"
)

// AllReferencedOutputs is the union of connector names across every profile.
func (c *Config) AllReferencedOutputs() []string {
	seen := make(map[string]struct{})
	var out []string
	for _, id := range c.OrderedIDs() {
		ref, err := ReferencedOutputs(c.Profiles[id].Monitors)
		if err != nil {
			continue
		}
		for _, name := range ref {
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, name)
		}
	}
	return out
}

// Upsert writes or replaces a profile and appends new ids to profile_order.
func (c *Config) Upsert(id, label string, monitors []string) error {
	if err := checkID(id); err != nil {
		return err
	}
	label = strings.TrimSpace(label)
	if label == "" {
		label = id
	}
	p := Profile{Label: label, Monitors: append([]string(nil), monitors...)}
	if !profileReferencesOutput(p, c.PrimaryMonitor) {
		return fmt.Errorf("profile %q must include a monitor line for primary_monitor %q", id, c.PrimaryMonitor)
	}
	if _, err := ActiveOutputs(p.Monitors); err != nil {
		return err
	}
	if _, exists := c.Profiles[id]; !exists {
		c.ProfileOrder = append(c.ProfileOrder, id)
	}
	if c.Profiles == nil {
		c.Profiles = map[string]Profile{}
	}
	c.Profiles[id] = p
	return nil
}

// Relabel changes only the display name.
func (c *Config) Relabel(id, label string) error {
	p, ok := c.Profiles[id]
	if !ok {
		return fmt.Errorf("unknown profile %q", id)
	}
	label = strings.TrimSpace(label)
	if label == "" {
		return fmt.Errorf("label is required")
	}
	p.Label = label
	c.Profiles[id] = p
	return nil
}

// Rename changes a profile key and updates profile_order.
func (c *Config) Rename(oldID, newID string) error {
	if oldID == newID {
		return checkID(newID)
	}
	if err := checkID(newID); err != nil {
		return err
	}
	p, ok := c.Profiles[oldID]
	if !ok {
		return fmt.Errorf("unknown profile %q", oldID)
	}
	if _, exists := c.Profiles[newID]; exists {
		return fmt.Errorf("profile %q already exists", newID)
	}
	c.Profiles[newID] = p
	delete(c.Profiles, oldID)
	replaced := false
	for i, id := range c.ProfileOrder {
		if id == oldID {
			c.ProfileOrder[i] = newID
			replaced = true
		}
	}
	if !replaced {
		c.ProfileOrder = append(c.ProfileOrder, newID)
	}
	return nil
}

// Delete removes a profile. The last remaining profile cannot be deleted.
func (c *Config) Delete(id string) error {
	if _, ok := c.Profiles[id]; !ok {
		return fmt.Errorf("unknown profile %q", id)
	}
	if len(c.Profiles) <= 1 {
		return fmt.Errorf("refusing to delete the last profile")
	}
	delete(c.Profiles, id)
	var order []string
	for _, existing := range c.ProfileOrder {
		if existing != id {
			order = append(order, existing)
		}
	}
	c.ProfileOrder = order
	return nil
}
