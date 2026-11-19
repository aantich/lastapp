import { Database, Plus } from "lucide-react";
import { Button, Card, EmptyState } from "../ui";

// The designer's empty state when the Space has no entities yet.
export function NoEntities({ create }: { create: () => void }) {
  return (
    <Card>
      <EmptyState
        icon={<Database size={28} aria-hidden />}
        title="No entities yet"
        actions={
          <Button
            variant="primary"
            icon={<Plus size={15} aria-hidden />}
            onClick={create}
          >
            New entity
          </Button>
        }
      >
        Describe your data once. Every App in this Space shares its API, access
        rules and validation.
      </EmptyState>
    </Card>
  );
}
